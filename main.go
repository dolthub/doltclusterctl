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
	"flag"
	"fmt"
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

const subcommands = `
SUBCOMMANDS

  doltclusterctl applyprimarylabels statefulset_name - sets/unsets the primary labels on the pods in the StatefulSet with metadata.name: statefulset-name; labels the other pods standby.
  doltclusterctl gracefulfailover statefulset_name - takes the current primary, marks it as a standby, and marks the next replica in the set as the primary.
  doltclusterctl promotestandby statefulset_name - takes the first reachable standby and makes it the new primary.
  doltclusterctl rollingrestart statefulset_name - deletes all pods in the stateful set, one at a time, waiting for the deleted pods to be recreated and ready before moving on; gracefully fails over the primary before deleting it.
`

func main() {
	var cfg Config
	cfg.Parse(flag.CommandLine, os.Args[1:])

	if cfg.TLSConfig != nil {
		mysql.RegisterTLSConfig("custom", cfg.TLSConfig)
	}

	fmt.Printf("running %s against %s/%s\n", cfg.CommandStr, cfg.Namespace, cfg.StatefulSetName)

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ctx, f := context.WithDeadline(context.TODO(), time.Now().Add(cfg.Timeout))
	defer f()
	if err := cfg.Command.Run(ctx, &cfg, clientset); err != nil {
		panic(err.Error())
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

func CallAssumeRole(ctx context.Context, db *sql.DB, role string, epoch int) error {
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

func LoadRoleAndEpoch(ctx context.Context, db *sql.DB) (string, int, error) {
	var role string
	var epoch int

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
			return false, fmt.Errorf("error updating pod %s/%s to add dolthub.com/cluster_role=primary label: %w", p.Namespace, p.Name, err)
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
			return false, fmt.Errorf("error updating pod %s/%s to add dolthub.com/cluster_role=standby label: %w", p.Namespace, p.Name, err)
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

	roles := make([]string, s.Replicas())
	epochs := make([]int, s.Replicas())
	errors := make([]error, s.Replicas())

	// Find current primary across the pods.
	for i := range s.Pods {
		err = func() error {
			db, err := s.DB(i)
			if err != nil {
				return err
			}
			defer db.Close()

			roles[i], epochs[i], errors[i] = LoadRoleAndEpoch(ctx, db)
			if errors[i] != nil {
				fmt.Printf("WARNING: error loading role and epoch for pod %s: %v", s.PodName(i), errors[i])
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	highestepoch := 0
	currentprimary := -1
	for i := range roles {
		if roles[i] == "primary" {
			if currentprimary != -1 {
				return fmt.Errorf("error apply primary labels, currently there is more than one primary, found pods: %s and %s", s.PodName(currentprimary), s.PodName(i))
			}
			currentprimary = i
		}
		if epochs[i] > highestepoch {
			highestepoch = epochs[i]
		}
	}

	if currentprimary == -1 {
		return fmt.Errorf("error did not find a pod that was in role primary, cannot apply labels")
	}

	// Apply the pod labels.
	for i, p := range s.Pods {
		if currentprimary == i {
			ok, err := LabelPrimary(ctx, s, i)
			if err != nil {
				return err
			}
			if ok {
				fmt.Printf("applied primary label to %s/%s\n", p.Namespace, p.Name)
			}
		} else {
			ok, err := LabelStandby(ctx, s, i)
			if err != nil {
				return err
			}
			if ok {
				fmt.Printf("removed primary label from %s/%s\n", p.Namespace, p.Name)
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
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	roles := make([]string, s.Replicas())
	epochs := make([]int, s.Replicas())

	// Find current primary across the pods.
	for i := range s.Pods {
		err = func() error {
			db, err := s.DB(i)
			if err != nil {
				return err
			}
			defer db.Close()

			roles[i], epochs[i], err = LoadRoleAndEpoch(ctx, db)
			if err != nil {
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
				return fmt.Errorf("error loading role and epoch for pod %s: %w", s.PodName(i), err)
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	highestepoch := 0
	currentprimary := -1
	for i := range roles {
		if roles[i] == "primary" {
			if currentprimary != -1 {
				return fmt.Errorf("error performing graceful failover, currently there is more than one primary, found pods: %s and %s", s.PodName(currentprimary), s.PodName(i))
			}
			currentprimary = i
		}
		if epochs[i] > highestepoch {
			highestepoch = epochs[i]
		}
	}

	if currentprimary == -1 {
		return fmt.Errorf("error did not find a pod that was in role primary, cannot perform graceful failover")
	}

	nextprimary := (currentprimary + 1) % len(s.Pods)
	nextepoch := highestepoch + 1

	fmt.Printf("failing over from %s to %s\n", s.PodName(currentprimary), s.PodName(nextprimary))

	for i := range s.Pods {
		_, err = LabelStandby(ctx, s, i)
		if err != nil {
			return err
		}
	}

	fmt.Printf("labeled all pods standby\n")

	err = func() error {
		db, err := s.DB(currentprimary)
		if err != nil {
			return err
		}
		defer db.Close()

		err = CallAssumeRole(ctx, db, "standby", nextepoch)
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}
	fmt.Printf("called dolt_assume_cluster_role standby on %s\n", s.PodName(currentprimary))

	err = func() error {
		db, err := s.DB(nextprimary)
		if err != nil {
			return err
		}
		defer db.Close()

		err = CallAssumeRole(ctx, db, "primary", nextepoch)
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}

	fmt.Printf("called dolt_assume_cluster_role primary on %s\n", s.PodName(nextprimary))

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}

	fmt.Printf("added primary label to %s\n", s.PodName(nextprimary))

	return nil
}

type PromoteStandby struct{}

func (cmd PromoteStandby) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	roles := make([]string, s.Replicas())
	epochs := make([]int, s.Replicas())

	// We ignore errors here, since we just want the first reachable standby.
	for i := range s.Pods {
		func() {
			db, err := s.DB(i)
			if err == nil {
				defer db.Close()
				roles[i], epochs[i], _ = LoadRoleAndEpoch(ctx, db)
			}
		}()
	}

	nextprimary := -1
	for i := range s.Pods {
		if roles[i] == "standby" {
			nextprimary = i
			break
		}
	}
	if nextprimary == -1 {
		return fmt.Errorf("failed to find a reachable standby to promote")
	}

	highestepoch := -1
	for i := range s.Pods {
		if epochs[i] > highestepoch {
			highestepoch = epochs[i]
		}
	}
	nextepoch := highestepoch + 1

	fmt.Printf("found standby to promote: %s\n", s.PodName(nextprimary))

	for i := range s.Pods {
		_, err = LabelStandby(ctx, s, i)
		if err != nil {
			return err
		}
	}

	fmt.Printf("labeled all pods as standby\n")

	err = func() error {
		db, err := s.DB(nextprimary)
		if err != nil {
			return err
		}
		defer db.Close()

		err = CallAssumeRole(ctx, db, "primary", nextepoch)
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}
	fmt.Printf("called dolt_assume_cluster_role primary on: %s\n", s.PodName(nextprimary))

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}
	fmt.Printf("applied primary label to %s\n", s.PodName(nextprimary))

	return nil
}

type RollingRestart struct {
}

func (RollingRestart) Run(ctx context.Context, cfg *Config, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, cfg, clientset)
	if err != nil {
		return err
	}
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", cfg.Namespace, cfg.StatefulSetName, len(s.Pods))

	dbstates := LoadDBStates(ctx, s)
	curprimary := -1
	highestepoch := -1
	for i := range dbstates {
		if dbstates[i].Err != nil {
			return fmt.Errorf("error loading role and epoch for %s: %w", s.PodName(i), dbstates[i].Err)
		}
		if dbstates[i].Role == "primary" {
			if curprimary != -1 {
				return fmt.Errorf("found more than one primary across the cluster, both %s and %s", s.PodName(curprimary), s.PodName(i))
			}
			curprimary = i
		}
		if dbstates[i].Role == "detected_broken_config" {
			return fmt.Errorf("found pod %s in detected_broken_config", s.PodName(i))
		}
		if dbstates[i].Epoch > highestepoch {
			highestepoch = dbstates[i].Epoch
		}
	}

	if curprimary == -1 {
		return fmt.Errorf("could not find a current primary across the cluster; cannot perform a rolling restart")
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
		fmt.Printf("pod is ready %s/%s\n", cfg.Namespace, s.Pods[i].Name)
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
	fmt.Printf("decided pod %s/%s will be next primary\n", cfg.Namespace, s.Pods[nextprimary].Name)

	_, err = LabelStandby(ctx, s, curprimary)
	if err != nil {
		return err
	}
	fmt.Printf("labeled existing primary, %s/%s as standby\n", cfg.Namespace, s.Pods[curprimary].Name)

	err = func() error {
		db, err := s.DB(curprimary)
		if err != nil {
			return err
		}
		defer db.Close()

		err = CallAssumeRole(ctx, db, "standby", nextepoch)
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}
	fmt.Printf("made existing primary, %s/%s role standby\n", cfg.Namespace, s.Pods[curprimary].Name)

	err = func() error {
		db, err := s.DB(nextprimary)
		if err != nil {
			return err
		}
		defer db.Close()

		err = CallAssumeRole(ctx, db, "primary", nextepoch)
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}
	fmt.Printf("made new primary, %s/%s, role primary\n", cfg.Namespace, s.Pods[nextprimary].Name)

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}
	fmt.Printf("labeled new primary, %s/%s, role primary\n", cfg.Namespace, s.Pods[nextprimary].Name)

	err = DeleteAndWaitForReady(ctx, s, curprimary)
	fmt.Printf("deleted old primary pod, %s/%s\n", cfg.Namespace, s.Pods[curprimary].Name)

	return err
}

type DBState struct {
	Role  string
	Epoch int
	Err   error
}

func LoadDBStates(ctx context.Context, s *State) []DBState {
	ret := make([]DBState, len(s.Pods))
	for i := range s.Pods {
		func() {
			db, err := s.DB(i)
			if err == nil {
				defer db.Close()
				ret[i].Role, ret[i].Epoch, ret[i].Err = LoadRoleAndEpoch(ctx, db)
			} else {
				ret[i].Err = err
			}
		}()
	}
	return ret
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
				fmt.Printf("poll for deletiong of pod %s/%s finished with ctx.Done()\n", s.Cfg.Namespace, s.Pods[i].Name)
				return
			}
		}
	}()
	fmt.Printf("deleting pod %s/%s\n", s.Cfg.Namespace, s.Pods[i].Name)
	err = pods.Delete(ctx, s.Pods[i].Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	<-done
	fmt.Printf("we believe pod %s/%s is deleted\n", s.Cfg.Namespace, s.Pods[i].Name)

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
	return fmt.Errorf("error: pod %s/%s did not become Ready after deleting it", s.Cfg.Namespace, s.Pods[i].Name)
}
