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
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
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

var namespace = flag.String("n", "default", "namespace of the stateful set to operate on")
var tlsCaPath = flag.String("tls-ca", "", "if provided, enables mandatory verified TLS mode; provides the path to a file to use as the certificate authority roots for verifying the server certificate")
var tlsServerName = flag.String("tls-server-name", "", "if provided, enables manadatory verified TLS mode and overrides the server name to verify as the CN or SAN of the leaf certificate (and present in SNI)")
var tlsVerified = flag.Bool("tls", false, "if provided, enables manadatory verified TLS mode")
var tlsInsecure = flag.Bool("tls-insecure", false, "if true, enables tls mode for communicating with the server, but does not verify the server's certificate")

var timeoutSecs = flag.Int("timeout-secs", 30, "the number of seconds the entire command has to run before it timeouts and exits non-zero")
var waitForReadySecs = flag.Int("wait-for-ready-secs", 50, "the number of seconds to wait for a single pod to become ready when performing a rollingrestart until we consider the operation failed")

var tlsConfigName string

var object string

const subcommands = `
SUBCOMMANDS

  doltclusterctl applyprimarylabels statefulset_name - sets/unsets the primary labels on the pods in the StatefulSet with metadata.name: statefulset-name; labels the other pods standby.
  doltclusterctl gracefulfailover statefulset_name - takes the current primary, marks it as a standby, and marks the next replica in the set as the primary.
  doltclusterctl promotestandby statefulset_name - takes the first reachable standby and makes it the new primary.
  doltclusterctl rollingrestart statefulset_name - deletes all pods in the stateful set, one at a time, waiting for the deleted pods to be recreated and ready before moving on; gracefully fails over the primary before deleting it.
`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s [COMMON OPTIONS...] subcommand statefulset_name\n\nCOMMON OPTIONS\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(flag.CommandLine.Output(), subcommands)
	}

	flag.Parse()

	if *tlsInsecure && *tlsVerified {
		fmt.Fprintf(flag.CommandLine.Output(), "Cannot provide -tls and -tls-insecure.\n\n")
		flag.Usage()
		os.Exit(2)
	}
	if *tlsInsecure && *tlsServerName != "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Cannot provide -tls-insecure and -tls-server-name.\n\n")
		flag.Usage()
		os.Exit(2)
	}
	if *tlsInsecure && *tlsCaPath != "" {
		fmt.Fprintf(flag.CommandLine.Output(), "Cannot provide -tls-insecure and -tls-ca.\n\n")
		flag.Usage()
		os.Exit(2)
	}

	if *tlsCaPath != "" || *tlsServerName != "" {
		// We register a custom TLS Config with the MySQL driver.
		cfg := &tls.Config{}
		if *tlsCaPath != "" {
			rootCertPool := x509.NewCertPool()
			pem, err := ioutil.ReadFile(*tlsCaPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to read tls-ca %s: %v\n", *tlsCaPath, err)
				os.Exit(1)
			}
			if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
				fmt.Fprintf(os.Stderr, "Failed to append PEM from %s.\n", *tlsCaPath)
				os.Exit(1)
			}
			cfg.RootCAs = rootCertPool
		}
		cfg.ServerName = *tlsServerName
		mysql.RegisterTLSConfig("custom", cfg)
		tlsConfigName = "custom"
	}

	cmdstr := flag.Arg(0)
	object = flag.Arg(1)

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	var cmd Command
	if cmdstr == "applyprimarylabels" {
		cmd = ApplyPrimaryLabels{}
	} else if cmdstr == "gracefulfailover" {
		cmd = GracefulFailover{}
	} else if cmdstr == "promotestandby" {
		cmd = PromoteStandby{}
	} else if cmdstr == "rollingrestart" {
		cmd = RollingRestart{}
	} else {
		fmt.Fprintf(flag.CommandLine.Output(), "Did not find subcommand %s\n\n", cmdstr)
		flag.Usage()
		os.Exit(2)
	}

	fmt.Printf("running %s against %s/%s\n", cmdstr, *namespace, object)

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ctx, f := context.WithDeadline(context.TODO(), time.Now().Add(time.Duration(*timeoutSecs)*time.Second))
	defer f()
	if err := cmd.Run(ctx, clientset); err != nil {
		panic(err.Error())
	}
}

type State struct {
	Clientset   *kubernetes.Clientset
	StatefulSet *appsv1.StatefulSet
	Pods        []*corev1.Pod
}

func LoadStatefulSet(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) (*State, error) {
	s := &State{Clientset: clientset}

	var err error
	s.StatefulSet, err = clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error loading StatefulSet %s/%s: %w", namespace, name, err)
	}

	s.Pods = make([]*corev1.Pod, s.Replicas())
	for i := range s.Pods {
		podname := name + "-" + strconv.Itoa(i)
		s.Pods[i], err = clientset.CoreV1().Pods(namespace).Get(ctx, podname, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error loading Pod %s/%s for StatefulSet %s/%s: %w", namespace, podname, namespace, name, err)
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
	dsn := RenderDSN(hostname, port)
	return sql.Open("mysql", dsn)
}

func RenderDSN(hostname string, port int) string {
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
	if *tlsInsecure {
		params += "?tls=skip-verify"
	} else if tlsConfigName != "" {
		params += "?tls=" + tlsConfigName
	} else if *tlsVerified {
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
		np, err := s.Clientset.CoreV1().Pods(*namespace).Update(ctx, p, metav1.UpdateOptions{})
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
		np, err := s.Clientset.CoreV1().Pods(*namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("error updating pod %s/%s to add dolthub.com/cluster_role=standby label: %w", p.Namespace, p.Name, err)
		}
		s.Pods[i] = np
	}
	return true, nil
}

type Command interface {
	Run(context.Context, *kubernetes.Clientset) error
}

type ApplyPrimaryLabels struct{}

func (cmd ApplyPrimaryLabels) Run(ctx context.Context, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, clientset, *namespace, object)
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

func (cmd GracefulFailover) Run(ctx context.Context, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, clientset, *namespace, object)
	if err != nil {
		return err
	}
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", *namespace, object, len(s.Pods))

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

func (cmd PromoteStandby) Run(ctx context.Context, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, clientset, *namespace, object)
	if err != nil {
		return err
	}
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", *namespace, object, len(s.Pods))

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

func (RollingRestart) Run(ctx context.Context, clientset *kubernetes.Clientset) error {
	s, err := LoadStatefulSet(ctx, clientset, *namespace, object)
	if err != nil {
		return err
	}
	fmt.Printf("loaded stateful set %s/%s with %d pods\n", *namespace, object, len(s.Pods))

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
		fmt.Printf("pod is ready %s/%s\n", *namespace, s.Pods[i].Name)
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
	fmt.Printf("decided pod %s/%s will be next primary\n", *namespace, s.Pods[nextprimary].Name)

	_, err = LabelStandby(ctx, s, curprimary)
	if err != nil {
		return err
	}
	fmt.Printf("labeled existing primary, %s/%s as standby\n", *namespace, s.Pods[curprimary].Name)

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
	fmt.Printf("made existing primary, %s/%s role standby\n", *namespace, s.Pods[curprimary].Name)

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
	fmt.Printf("made new primary, %s/%s, role primary\n", *namespace, s.Pods[nextprimary].Name)

	_, err = LabelPrimary(ctx, s, nextprimary)
	if err != nil {
		return err
	}
	fmt.Printf("labeled new primary, %s/%s, role primary\n", *namespace, s.Pods[nextprimary].Name)

	err = DeleteAndWaitForReady(ctx, s, curprimary)
	fmt.Printf("deleted old primary pod, %s/%s\n", *namespace, s.Pods[curprimary].Name)

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
	pods := s.Clientset.CoreV1().Pods(*namespace)
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
				fmt.Printf("poll for deletiong of pod %s/%s finished with ctx.Done()\n", *namespace, s.Pods[i].Name)
				return
			}
		}
	}()
	fmt.Printf("deleting pod %s/%s\n", *namespace, s.Pods[i].Name)
	err = pods.Delete(ctx, s.Pods[i].Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	<-done
	fmt.Printf("we believe pod %s/%s is deleted\n", *namespace, s.Pods[i].Name)

	pollInterval := 100 * time.Millisecond
	deadline := time.Now().Add(time.Duration(*waitForReadySecs) * time.Second)
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
	return fmt.Errorf("error: pod %s/%s did not become Ready after deleting it", *namespace, s.Pods[i].Name)
}
