// Copyright 2022 DoltHub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
)

type Command interface {
	Run(context.Context, *Config, Cluster) error
}

type ApplyPrimaryLabels struct{}

func (cmd ApplyPrimaryLabels) Run(ctx context.Context, cfg *Config, cluster Cluster) error {
	dbstates := LoadDBStates(ctx, cfg, cluster)
	for _, state := range dbstates {
		if state.Err != nil {
			log.Printf("WARNING: error loading role and epoch for pod %s: %v", state.Instance.Name(), state.Err)
		}
	}

	// Find current primary across the pods.
	currentprimary, _, err := CurrentPrimaryAndEpoch(dbstates)
	if err != nil {
		return fmt.Errorf("cannot apply primary labels: %w", err)
	}

	// Apply the pod labels.
	for i, state := range dbstates {
		instance := state.Instance
		if currentprimary == i {
			if instance.Role() != RolePrimary {
				err := instance.MarkRolePrimary(ctx)
				if err != nil {
					return err
				}
				log.Printf("applied primary label to %s", instance.Name())
			}
		} else {
			if instance.Role() != RoleStandby {
				err := instance.MarkRoleStandby(ctx)
				if err != nil {
					return err
				}
				log.Printf("applied standby label to %s", instance.Name())
			}
		}
	}

	return nil
}

type GracefulFailover struct{}

func (cmd GracefulFailover) Run(ctx context.Context, cfg *Config, cluster Cluster) error {
	dbstates := LoadDBStates(ctx, cfg, cluster)
	for _, state := range dbstates {
		if state.Err != nil {
			// TODO: For now this remains an error.
			// GracefulFailover is going to require all
			// standbys to be caught up on all databases.
			// If one of the databases is down, this is
			// going to fail. Better to not disrupt traffic
			// in this case.
			//
			// Once we can coordinate
			// dolt_assume_role('standby', ...) to only
			// need to true up 2/n+1 replicas, for example,
			// and doltclusterctl to pick a standby to
			// become the new primary which is recent
			// enough, this error can change.
			return fmt.Errorf("error loading role and epoch for pod %s: %w", state.Instance.Name(), state.Err)
		}
	}

	// Find current primary across the pods.
	currentprimary, highestepoch, err := CurrentPrimaryAndEpoch(dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform graceful failover: %w", err)
	}

	nextprimary := (currentprimary + 1) % cluster.NumReplicas()
	nextepoch := highestepoch + 1

	oldPrimary := dbstates[currentprimary].Instance
	newPrimary := dbstates[nextprimary].Instance

	log.Printf("failing over from %s to %s", oldPrimary.Name(), newPrimary.Name())

	for _, state := range dbstates {
		err := state.Instance.MarkRoleStandby(ctx)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods standby")

	err = CallAssumeRole(ctx, cfg, oldPrimary, "standby", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("called dolt_assume_cluster_role standby on %s", oldPrimary.Name())

	err = CallAssumeRole(ctx, cfg, newPrimary, "primary", nextepoch)
	if err != nil {
		return err
	}

	log.Printf("called dolt_assume_cluster_role primary on %s", newPrimary.Name())

	err = newPrimary.MarkRolePrimary(ctx)
	if err != nil {
		return err
	}

	log.Printf("added primary label to %s", newPrimary.Name())

	return nil
}

type PromoteStandby struct{}

func (cmd PromoteStandby) Run(ctx context.Context, cfg *Config, cluster Cluster) error {
	// We ignore errors here, since we just want the first reachable standby.
	dbstates := LoadDBStates(ctx, cfg, cluster)

	nextprimary := -1
	for i, state := range dbstates {
		if state.Role == "standby" {
			nextprimary = i
			break
		}
	}
	if nextprimary == -1 {
		return fmt.Errorf("failed to find a reachable standby to promote")
	}

	highestepoch := -1
	for _, state := range dbstates {
		if state.Epoch > highestepoch {
			highestepoch = state.Epoch
		}
	}
	nextepoch := highestepoch + 1

	newPrimary := dbstates[nextprimary].Instance

	log.Printf("found standby to promote: %s", newPrimary.Name())

	for _, state := range dbstates {
		instance := state.Instance
		err := instance.MarkRoleStandby(ctx)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods as standby")

	err := CallAssumeRole(ctx, cfg, newPrimary, "primary", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("called dolt_assume_cluster_role primary on %s", newPrimary.Name())

	err = newPrimary.MarkRolePrimary(ctx)
	if err != nil {
		return err
	}
	log.Printf("applied primary label to %s", newPrimary.Name())

	return nil
}

func WaitForDBReady(ctx context.Context, cfg *Config, instance Instance) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		db, err := OpenDB(ctx, cfg, instance)
		if err != nil {
			continue
		}
		err = db.PingContext(ctx)
		if err == nil {
			return nil
		}
	}
}

type RollingRestart struct {
}

func (RollingRestart) Run(ctx context.Context, cfg *Config, cluster Cluster) error {
	dbstates := LoadDBStates(ctx, cfg, cluster)

	for _, state := range dbstates {
		if state.Err != nil {
			return fmt.Errorf("cannot perform rolling restart: %w", state.Err)
		}
		if state.Role == "detected_broken_config" {
			return fmt.Errorf("cannot perform rolling restart: found pod %s in detected_broken_config", state.Instance.Name())
		}
	}

	curprimary, highestepoch, err := CurrentPrimaryAndEpoch(dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform rolling restart: %w", err)
	}

	nextepoch := highestepoch + 1

	// In order from highest ordinal to lowest, we are going to restart each standby...
	for i := len(dbstates) - 1; i >= 0; i-- {
		if i == curprimary {
			continue
		}
		state := dbstates[i]
		instance := state.Instance

		restartCtx, cancel := context.WithTimeout(ctx, cfg.WaitForReady)
		defer cancel()
		err := instance.Restart(restartCtx)
		if err != nil {
			return err
		}

		err = WaitForDBReady(restartCtx, cfg, instance)
		if err != nil {
			return err
		}

		log.Printf("pod is ready %s", instance.Name())
	}

	// Every standby has been restarted. We failover the primary to the
	// lowest-ordinal standby pod and then restart the primary.
	nextprimary := -1
	for i := range dbstates {
		if dbstates[i].Role == "standby" {
			nextprimary = i
			break
		}
	}
	if nextprimary == -1 {
		return fmt.Errorf("failed to find a reachable standby to promote")
	}

	oldPrimary := dbstates[curprimary].Instance
	newPrimary := dbstates[nextprimary].Instance

	log.Printf("decided pod %s will be next primary", newPrimary.Name())

	err = oldPrimary.MarkRoleStandby(ctx)
	if err != nil {
		return err
	}
	log.Printf("labeled existing primary, %s, as standby", oldPrimary.Name())

	err = CallAssumeRole(ctx, cfg, oldPrimary, "standby", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("made existing primary, %s, role standby", oldPrimary.Name())

	err = CallAssumeRole(ctx, cfg, newPrimary, "primary", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("made new primary, %s, role primary", newPrimary.Name())

	err = newPrimary.MarkRolePrimary(ctx)
	if err != nil {
		return err
	}
	log.Printf("labeled new primary, %s, role primary", newPrimary.Name())

	// Finally restart the old primary.

	restartCtx, cancel := context.WithTimeout(ctx, cfg.WaitForReady)
	defer cancel()
	err = oldPrimary.Restart(restartCtx)
	if err != nil {
		return err
	}

	err = WaitForDBReady(restartCtx, cfg, oldPrimary)
	if err != nil {
		return err
	}

	log.Printf("pod is ready %s", oldPrimary.Name())

	return nil
}
