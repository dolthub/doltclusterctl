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
	"time"
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

	var errStates []DBState
	for _, state := range dbstates {
		if state.Err != nil {
			errStates = append(errStates, state)
		}
	}

	if len(errStates) > 0 && cfg.MinCaughtUpStandbys == -1 {
		// If we need to catch up all standbys, but we
		// can't currently reach one of the standbys,
		// something is wrong. We do not go forward with
		// the attempt.
		return fmt.Errorf("error loading role and epoch for pod %s: %w", errStates[0].Instance.Name(), errStates[0].Err)
	}

	numStandbys := len(dbstates) - 1
	numReachableStandbys := len(dbstates) - len(errStates) - 1

	if cfg.MinCaughtUpStandbys != -1 {
		if numStandbys < cfg.MinCaughtUpStandbys {
			return fmt.Errorf("Invalid min-caughtup-standbys of %d provided. Only %d pods are in the cluster, so only %d standbys can ever be caught up.", cfg.MinCaughtUpStandbys, len(dbstates), len(dbstates)-1)
		}
		if numReachableStandbys < cfg.MinCaughtUpStandbys {
			return fmt.Errorf("could not reach enough standbys to catch up %d. Out of %d pods, %d were unreachable. For example, contacting pod %s resulted in error: %w", cfg.MinCaughtUpStandbys, len(dbstates), len(errStates), errStates[0].Instance.Name(), errStates[0].Err)
		}
	}

	// Find current primary across the pods.
	currentprimary, highestepoch, err := CurrentPrimaryAndEpoch(dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform graceful failover: %w", err)
	}

	oldPrimary := dbstates[currentprimary].Instance
	nextepoch := highestepoch + 1

	if cfg.MinCaughtUpStandbys != -1 && !VersionSupportsTransitionToStandby(dbstates[currentprimary].Version) {
		return fmt.Errorf("Cannot perform gracefulfailover with min-caughtup-standbys of %d. The version of Dolt on the current primary (%s on pod %s) does not support dolt_cluster_transition_to_standby.", cfg.MinCaughtUpStandbys, dbstates[currentprimary].Version, oldPrimary.Name())
	}

	log.Printf("failing over from %s", oldPrimary.Name())

	for _, state := range dbstates {
		err := state.Instance.MarkRoleStandby(ctx)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods standby")

	var newPrimary Instance

	if cfg.MinCaughtUpStandbys == -1 {
		nextprimary := (currentprimary + 1) % cluster.NumReplicas()
		newPrimary = dbstates[nextprimary].Instance

		err = CallAssumeRole(ctx, cfg, oldPrimary, "standby", nextepoch)
		if err != nil {
			log.Printf("failed to transition primary to standby. labeling old primary as primary.")
			err = oldPrimary.MarkRolePrimary(ctx)
			if err != nil {
				log.Printf("ERROR: failed to label old primary as primary.")
				log.Printf("\t%v", err)
				log.Printf("dolt-rw endpoint will be broken. You need to run applyprimarylabels.")
			}
			return fmt.Errorf("error calling dolt_assume_cluster_role standby on %s: %w", oldPrimary.Name(), err)
		}
		log.Printf("called dolt_assume_cluster_role standby on %s", oldPrimary.Name())
	} else {
		nextprimary, err := CallTransitionToStandby(ctx, cfg, oldPrimary, nextepoch, dbstates)
		if err != nil {
			log.Printf("failed to transition primary to standby. labeling old primary as primary.")
			err = oldPrimary.MarkRolePrimary(ctx)
			if err != nil {
				log.Printf("ERROR: failed to label old primary as primary.")
				log.Printf("\t%v", err)
				log.Printf("dolt-rw endpoint will be broken. You need to run applyprimarylabels.")
			}
			return fmt.Errorf("error calling dolt_cluster_transition_to_standby on %s: %w", oldPrimary.Name(), err)
		}
		log.Printf("called dolt_cluster_transition_to_standby on %s", oldPrimary.Name())
		newPrimary = dbstates[nextprimary].Instance
	}

	log.Printf("failing over to %s", newPrimary.Name())

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

func PickNextPrimary(dbstates []DBState) int {
	firststandby := -1
	nextprimary := -1
	var updated time.Time
	for i, state := range dbstates {
		if state.Role == "standby" {
			if firststandby == -1 {
				firststandby = i
			}

			var oldestDB time.Time
			for _, status := range state.Status {
				if status.LastUpdate.Valid && (oldestDB == (time.Time{}) || oldestDB.After(status.LastUpdate.Time)) {
					oldestDB = status.LastUpdate.Time
				}
			}

			if oldestDB != (time.Time{}) && (updated == (time.Time{}) || updated.Before(oldestDB)) {
				nextprimary = i
				updated = oldestDB
			}
		}
	}
	if nextprimary != -1 {
		return nextprimary
	}
	return firststandby
}

type PromoteStandby struct{}

func (cmd PromoteStandby) Run(ctx context.Context, cfg *Config, cluster Cluster) error {
	// We ignore errors here, since we just want the first reachable standby.
	dbstates := LoadDBStates(ctx, cfg, cluster)

	nextprimary := PickNextPrimary(dbstates)
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

		// We need to relabel the pod, since we deleted it.
		err = instance.MarkRoleStandby(ctx)
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

	// We need to relabel the pod, since we deleted it.
	err = oldPrimary.MarkRoleStandby(ctx)
	if err != nil {
		return err
	}

	log.Printf("pod is ready %s", oldPrimary.Name())

	return nil
}
