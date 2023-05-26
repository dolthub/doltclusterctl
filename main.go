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
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var cfg Config
	cfg.Parse(flag.CommandLine, os.Args[1:])

	if cfg.TLSConfig != nil {
		mysql.RegisterTLSConfig("custom", cfg.TLSConfig)
	}

	log.Printf("running %s against %s/%s", cfg.CommandStr, cfg.Namespace, cfg.StatefulSetName)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("could not load kubernetes InClusterConfig: %v", err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("could not build kubernetes client for config: %v", err.Error())
	}

	ctx, f := context.WithDeadline(context.TODO(), time.Now().Add(cfg.Timeout))
	defer f()
	if err := cfg.Command.Run(ctx, &cfg, clientset); err != nil {
		log.Fatalf("error running command: %v", err.Error())
	}
}

type State struct {
	Cfg         *Config
	Clientset   *kubernetes.Clientset
	StatefulSet *appsv1.StatefulSet
	Pods        []*corev1.Pod
}

func LoadStatefulSet(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) (*State, error) {
	s := &State{
		Cfg:       cfg,
		Clientset: clientset,
	}

	var err error
	s.StatefulSet, err = clientset.AppsV1().StatefulSets(cfg.Namespace).Get(ctx, cfg.StatefulSetName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error loading StatefulSet %s/%s: %w", cfg.Namespace, cfg.StatefulSetName, err)
	}

	s.Pods = make([]*corev1.Pod, s.Replicas())
	for i := range s.Pods {
		podname := cfg.StatefulSetName + "-" + strconv.Itoa(i)
		s.Pods[i], err = clientset.CoreV1().Pods(cfg.Namespace).Get(ctx, podname, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error loading Pod %s/%s for StatefulSet %s/%s: %w", cfg.Namespace, podname, cfg.Namespace, cfg.StatefulSetName, err)
		}
	}

	return s, nil
}

func (s *State) Replicas() int {
	if s.StatefulSet.Spec.Replicas != nil {
		return int(*s.StatefulSet.Spec.Replicas)
	}
	return 1
}

func (s *State) PodHostname(i int) string {
	return s.Pods[i].Name + "." + s.ServiceName() + "." + s.Pods[i].Namespace
}

func (s *State) PodName(i int) string {
	return s.Pods[i].Namespace + "/" + s.Pods[i].Name
}

func (s *State) Port() int {
	for _, c := range s.StatefulSet.Spec.Template.Spec.Containers {
		if c.Name == "dolt" {
			for _, p := range c.Ports {
				if p.Name == "dolt" {
					return int(p.ContainerPort)
				}
			}
		}
	}
	return 3306
}

func (s *State) ServiceName() string {
	return s.StatefulSet.Spec.ServiceName
}

func (s *State) DB(i int) (*sql.DB, error) {
	hostname := s.PodHostname(i)
	port := s.Port()
	dsn := RenderDSN(s.Cfg, hostname, port)
	return sql.Open("mysql", dsn)
}

func RenderDSN(cfg *Config, hostname string, port int) string {
	user := os.Getenv("DOLT_USERNAME")
	if user == "" {
		user = "root"
	}
	authority := user
	pass := os.Getenv("DOLT_PASSWORD")
	if pass != "" {
		authority += ":" + pass
	}
	params := ""
	if cfg.TLSInsecure {
		params += "?tls=skip-verify"
	} else if cfg.TLSConfig != nil {
		// TODO: This is spookily coupled to the config name in main
		params += "?tls=custom"
	} else if cfg.TLSVerified {
		params += "?tls=true"
	}
	return fmt.Sprintf("%s@tcp(%s:%d)/dolt_cluster%s", authority, hostname, port, params)
}

func CallAssumeRole(ctx context.Context, s *State, i int, role string, epoch int) error {
	db, err := s.DB(i)
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var status int

	q := fmt.Sprintf("CALL DOLT_ASSUME_CLUSTER_ROLE('%s', %d)", role, epoch)
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&status)
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("result from call dolt_assume_role('%s', %d) was %d, not 0", role, epoch, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

func LoadRoleAndEpoch(ctx context.Context, s *State, i int) (string, int, error) {
	var role string
	var epoch int

	db, err := s.DB(i)
	if err != nil {
		return "", 0, err
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return "", 0, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, "SELECT @@global.dolt_cluster_role, @@global.dolt_cluster_role_epoch")
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&role, &epoch)
		if err != nil {
			return "", 0, err
		}
	}
	if rows.Err() != nil {
		return "", 0, rows.Err()
	}

	return role, epoch, nil
}

const RoleLabel = "dolthub.com/cluster_role"
const PrimaryRoleValue = "primary"
const StandbyRoleValue = "standby"

func LabelPrimary(ctx context.Context, s *State, i int) (bool, error) {
	p := s.Pods[i]
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == PrimaryRoleValue {
		// Do not need to do anything...
		return false, nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = PrimaryRoleValue
		np, err := s.Clientset.CoreV1().Pods(s.Cfg.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("error updating pod %s to add dolthub.com/cluster_role=primary label: %w", s.PodName(i), err)
		}
		s.Pods[i] = np
	}
	return true, nil
}

func LabelStandby(ctx context.Context, s *State, i int) (bool, error) {
	p := s.Pods[i]
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == StandbyRoleValue {
		// Do not need to do anything...
		return false, nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = StandbyRoleValue
		np, err := s.Clientset.CoreV1().Pods(s.Cfg.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("error updating pod %s to add dolthub.com/cluster_role=standby label: %w", s.PodName(i), err)
		}
		s.Pods[i] = np
	}
	return true, nil
}

type Command interface {
	Run(context.Context, *Config, *kubernetes.Clientset) error
}

type ApplyPrimaryLabels struct{}

func (cmd ApplyPrimaryLabels) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	log.Printf("loaded stateful set %s/%s with %d pods", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	dbstates := LoadDBStates(ctx, s)
	for i, state := range dbstates {
		if state.Err != nil {
			log.Printf("WARNING: error loading role and epoch for pod %s: %v", s.PodName(i), state.Err)
		}
	}

	// Find current primary across the pods.
	currentprimary, _, err := CurrentPrimaryAndEpoch(s, dbstates)
	if err != nil {
		return fmt.Errorf("cannot apply primary labels: %w", err)
	}

	// Apply the pod labels.
	for i := range s.Pods {
		if currentprimary == i {
			ok, err := LabelPrimary(ctx, s, i)
			if err != nil {
				return err
			}
			if ok {
				log.Printf("applied primary label to %s", s.PodName(i))
			}
		} else {
			ok, err := LabelStandby(ctx, s, i)
			if err != nil {
				return err
			}
			if ok {
				log.Printf("applied standby label to %s", s.PodName(i))
			}
		}
	}

	return nil
}

type GracefulFailover struct{}

func (cmd GracefulFailover) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	log.Printf("loaded stateful set %s/%s with %d pods", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	dbstates := LoadDBStates(ctx, s)
	for i, state := range dbstates {
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
			return fmt.Errorf("error loading role and epoch for pod %s: %w", s.PodName(i), state.Err)
		}
	}

	// Find current primary across the pods.

	currentprimary, highestepoch, err := CurrentPrimaryAndEpoch(s, dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform graceful failover: %w", err)
	}

	nextprimary := (currentprimary + 1) % len(s.Pods)
	nextepoch := highestepoch + 1

	log.Printf("failing over from %s to %s", s.PodName(currentprimary), s.PodName(nextprimary))

	for i := range s.Pods {
		_, err = LabelStandby(ctx, s, i)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods standby")

	err = CallAssumeRole(ctx, s, currentprimary, "standby", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("called dolt_assume_cluster_role standby on %s", s.PodName(currentprimary))

	err = CallAssumeRole(ctx, s, nextprimary, "primary", nextepoch)
	if err != nil {
		return err
	}

	log.Printf("called dolt_assume_cluster_role primary on %s", s.PodName(nextprimary))

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}

	log.Printf("added primary label to %s", s.PodName(nextprimary))

	return nil
}

type PromoteStandby struct{}

func (cmd PromoteStandby) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	log.Printf("loaded stateful set %s/%s with %d pods", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	// We ignore errors here, since we just want the first reachable standby.
	dbstates := LoadDBStates(ctx, s)

	nextprimary := -1
	for i := range s.Pods {
		if dbstates[i].Role == "standby" {
			nextprimary = i
			break
		}
	}
	if nextprimary == -1 {
		return fmt.Errorf("failed to find a reachable standby to promote")
	}

	highestepoch := -1
	for i := range s.Pods {
		if dbstates[i].Epoch > highestepoch {
			highestepoch = dbstates[i].Epoch
		}
	}
	nextepoch := highestepoch + 1

	log.Printf("found standby to promote: %s", s.PodName(nextprimary))

	for i := range s.Pods {
		_, err = LabelStandby(ctx, s, i)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods as standby")

	err = CallAssumeRole(ctx, s, nextprimary, "primary", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("called dolt_assume_cluster_role primary on %s", s.PodName(nextprimary))

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}
	log.Printf("applied primary label to %s", s.PodName(nextprimary))

	return nil
}

type RollingRestart struct {
}

func (RollingRestart) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	log.Printf("loaded stateful set %s/%s with %d pods", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	dbstates := LoadDBStates(ctx, s)

	for i := range dbstates {
		if dbstates[i].Err != nil {
			return fmt.Errorf("cannot perform rolling restart: %w", dbstates[i].Err)
		}
		if dbstates[i].Role == "detected_broken_config" {
			return fmt.Errorf("cannot perform rolling restart: found pod %s in detected_broken_config", s.PodName(i))
		}
	}

	curprimary, highestepoch, err := CurrentPrimaryAndEpoch(s, dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform rolling restart: %w", err)
	}

	nextepoch := highestepoch + 1

	// In order from highest ordinal to lowest, we are going to restart each standby...
	for i := len(dbstates) - 1; i >= 0; i-- {
		if i == curprimary {
			continue
		}

		err := DeleteAndWaitForReady(ctx, s, i)
		if err != nil {
			return err
		}
		log.Printf("pod is ready %s", s.PodName(i))
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
	log.Printf("decided pod %s will be next primary", s.PodName(nextprimary))

	_, err = LabelStandby(ctx, s, curprimary)
	if err != nil {
		return err
	}
	log.Printf("labeled existing primary, %s, as standby", s.PodName(curprimary))

	err = CallAssumeRole(ctx, s, curprimary, "standby", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("made existing primary, %s, role standby", s.PodName(curprimary))

	err = CallAssumeRole(ctx, s, nextprimary, "primary", nextepoch)
	if err != nil {
		return err
	}
	log.Printf("made new primary, %s, role primary", s.PodName(nextprimary))

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}
	log.Printf("labeled new primary, %s, role primary", s.PodName(nextprimary))

	err = DeleteAndWaitForReady(ctx, s, curprimary)
	if err != nil {
		return err
	}
	log.Printf("pod is ready %s", s.PodName(curprimary))

	return nil
}

type DBState struct {
	Role  string
	Epoch int
	Err   error
}

func LoadDBStates(ctx context.Context, s *State) []DBState {
	ret := make([]DBState, len(s.Pods))
	for i := range s.Pods {
		ret[i].Role, ret[i].Epoch, ret[i].Err = LoadRoleAndEpoch(ctx, s, i)
		if ret[i].Err != nil {
			ret[i].Err = fmt.Errorf("error loading role and epoch for %s: %w", s.PodName(i), ret[i].Err)
		}
	}
	return ret
}

// Returns the current valid primary based on the dbstates. Returns an error if
// there is no primary or if there is more than one primary.
func CurrentPrimaryAndEpoch(s *State, dbstates []DBState) (int, int, error) {
	highestepoch := 0
	currentprimary := -1
	for i := range dbstates {
		if dbstates[i].Role == "primary" {
			if currentprimary != -1 {
				return -1, -1, fmt.Errorf("more than one reachable pod was in role primary: %s and %s", s.PodName(currentprimary), s.PodName(i))
			}
			currentprimary = i
		}
		if dbstates[i].Epoch > highestepoch {
			highestepoch = dbstates[i].Epoch
		}
	}

	if currentprimary == -1 {
		return -1, -1, errors.New("no reachable pod was in role primary")
	}

	return currentprimary, highestepoch, nil
}

func DeleteAndWaitForReady(ctx context.Context, s *State, i int) error {
	pods := s.Clientset.CoreV1().Pods(s.Cfg.Namespace)
	w, err := pods.Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", s.Pods[i].Name).String(),
	})
	if err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		defer w.Stop()
		defer close(done)
		for {
			select {
			case r := <-w.ResultChan():
				if r.Type == watch.Deleted {
					return
				}
			case <-ctx.Done():
				log.Printf("poll for deletiong of pod %s finished with ctx.Done()", s.PodName(i))
				return
			}
		}
	}()
	log.Printf("deleting pod %s", s.PodName(i))
	err = pods.Delete(ctx, s.Pods[i].Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	<-done
	log.Printf("pod %s successfully deleted", s.PodName(i))

	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(s.Cfg.WaitForReady)
PollPod:
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		p, err := pods.Get(ctx, s.Pods[i].Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		for _, c := range p.Status.ContainerStatuses {
			if !c.Ready {
				continue PollPod
			}
		}

		err = func() error {
			db, err := s.DB(i)
			if err == nil {
				defer db.Close()
				ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()
				err = db.PingContext(ctx)
			}
			return err
		}()
		if err != nil {
			continue
		}

		// If we get here, pod exists and all its containers are Ready.
		s.Pods[i] = p
		return nil
	}
	return fmt.Errorf("error: pod %s did not become Ready after deleting it", s.PodName(i))
}
