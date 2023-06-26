// Copyright 2023 DoltHub, Inc.
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
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestApplyPrimaryLabels(t *testing.T) {
	feature := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	password := features.New("WithPassword").
		WithSetup("create statefulset", CreateStatefulSet(WithCredentials(envconf.RandomName("user", 12), envconf.RandomName("pass", 12)))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	tlsinsecureagainstplaintext := features.New("TLSInsecureAgainstPlaintextServer").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(
			WithArgs("-tls-insecure", "applyprimarylabels", "dolt"),
			ShouldFailWith("TLS requested but server does not support TLS"))).
		Feature()
	tlsinsecureagainsttlsloose := features.New("TLSInsecureAgainstTLSRequired").
		WithSetup("create statefulset", CreateStatefulSet(WithTLSMode(TLSModeOptional))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("-tls-insecure", "applyprimarylabels", "dolt"))).
		Feature()
	tlsca := features.New("TLSCA").
		WithSetup("create statefulset", CreateStatefulSet(WithTLSMode(TLSModeRequired))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("-tls-ca", "/etc/doltcerts/roots.pem", "applyprimarylabels", "dolt"), WithTLSRoots())).
		Feature()
	wrongtlsservername := features.New("WrongTLSServerName").
		WithSetup("create statefulset", CreateStatefulSet(WithTLSMode(TLSModeRequired))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(
			WithArgs("-tls-ca", "/etc/doltcerts/roots.pem", "-tls-server-name", "does.not.match", "applyprimarylabels", "dolt"),
			WithTLSRoots(),
			ShouldFailWith("tls: failed to verify certificate: x509: certificate is valid for "))).
		Feature()

	testenv.Test(t, feature, password, tlsinsecureagainstplaintext, tlsinsecureagainsttlsloose, tlsca, wrongtlsservername)
}

type DCCJob struct {
	Args         []string
	FailMatch    string
	WithTLSRoots bool
}

type DCCJobOption func(*DCCJob)

func WithArgs(args ...string) DCCJobOption {
	return func(job *DCCJob) {
		job.Args = args
	}
}

func WithTLSRoots() DCCJobOption {
	return func(job *DCCJob) {
		job.WithTLSRoots = true
	}
}

func ShouldFailWith(match string) DCCJobOption {
	return func(job *DCCJob) {
		job.FailMatch = match
	}
}

func RunDoltClusterCtlJob(opts ...DCCJobOption) features.Func {
	var dccjob DCCJob
	for _, o := range opts {
		o(&dccjob)
	}
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		name := envconf.RandomName("doltclusterctl", 24)

		state, _ := GetStatefulSet(ctx)
		config := state.Config

		// Create the job.
		job := NewDoltClusterCtlJob(name, c.Namespace(), config, dccjob)

		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		err = client.Resources().Create(context.TODO(), job)
		if err != nil {
			t.Fatal(err)
		}

		// Delete the job.
		defer func() {
			err = client.Resources().Delete(context.TODO(), job)
			if err != nil {
				t.Logf("WARN: failed to delete job %v: %v", name, err)
			}
		}()

		// Wait for it to complete.
		var success bool
		err = wait.For(JobCompletedOrFailed(client.Resources(), job, &success), wait.WithTimeout(time.Minute*1))
		if err != nil {
			t.Fatal(err)
		}

		if success && dccjob.FailMatch != "" {
			t.Fatalf("expected job to fail with '%s' but the job succeeded", dccjob.FailMatch)
		}

		if !success {
			// TODO: We need to assert on the fail match in the logs for the pod for the job.
			podselector := job.Spec.Selector
			var pods v1.PodList
			err = client.Resources(c.Namespace()).List(context.TODO(), &pods, resources.WithLabelSelector(labels.Set(podselector.MatchLabels).String()))
			if err != nil {
				t.Fatalf("unable to list pods for failed job: %v", err)
			}
			if len(pods.Items) != 1 {
				t.Fatalf("listing pods for failed job expected to find 1 pod, found: %d", len(pods.Items))
			}
			pod := pods.Items[0]

			clientset, err := kubernetes.NewForConfig(client.RESTConfig())
			if err != nil {
				t.Fatalf("could not create client for retrieving pod logs: %v", err)
			}
			var podLogOpts v1.PodLogOptions
			req := clientset.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &podLogOpts)
			stream, err := req.Stream(context.TODO())
			if err != nil {
				t.Fatalf("error retreiving pod logs: %v", err)
			}
			defer stream.Close()
			var contents bytes.Buffer
			_, err = io.Copy(&contents, stream)
			if err != nil {
				t.Fatalf("error retreiving pod logs: %v", err)
			}

			contentsStr := contents.String()
			if dccjob.FailMatch != "" {
				if !strings.Contains(contentsStr, dccjob.FailMatch) {
					t.Fatalf("failed to find expected match '%s' in pod logs:\n%s", dccjob.FailMatch, contentsStr)
				}
			} else {
				t.Fatalf("expected job to succeed but it failed:\n%s", contentsStr)
			}
		}

		return ctx
	}
}

func JobCompletedOrFailed(resources *resources.Resources, job k8s.Object, success *bool) apimachinerywait.ConditionFunc {
	return func() (done bool, err error) {
		if err := resources.Get(context.TODO(), job.GetName(), job.GetNamespace(), job); err != nil {
			return false, err
		}
		status := job.(*batchv1.Job).Status
		for _, cond := range status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == v1.ConditionTrue {
				done = true
				*success = false
			} else if cond.Type == batchv1.JobComplete && cond.Status == v1.ConditionTrue {
				done = true
				*success = true
			}
		}
		return
	}
}

func NewDoltClusterCtlJob(name, namespace string, config StatefulSetConfig, dccjob DCCJob) *batchv1.Job {
	labels := map[string]string{"app": "doltclusterctl", "job": name}
	// In real life, these would ValueFrom a secret, but this is fine for the test, at least for now.
	var env []v1.EnvVar
	if config.Username != "" {
		env = append(env, v1.EnvVar{
			Name:  "DOLT_USERNAME",
			Value: config.Username,
		})
	}
	if config.Password != "" {
		env = append(env, v1.EnvVar{
			Name:  "DOLT_PASSWORD",
			Value: config.Password,
		})
	}
	var backoff int32 = 0
	ret := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					ServiceAccountName: "doltclusterctl",
					Containers: []v1.Container{{
						Name:            "doltclusterctl",
						Image:           DoltClusterCtlImage,
						ImagePullPolicy: v1.PullNever,
						Command:         append([]string{"/usr/local/bin/doltclusterctl", "-n", namespace}, dccjob.Args...),
						Env:             env,
					}},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	if dccjob.WithTLSRoots {
		ret.Spec.Template.Spec.Volumes = []v1.Volume{{
			Name: "tls-roots",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "dolt-roots",
					},
				},
			},
		}}

		ret.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{
			Name:      "tls-roots",
			MountPath: "/etc/doltcerts",
		}}
	}

	return ret
}

func AssertPodHasLabel(name, key, value string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		var pod v1.Pod
		err = client.Resources().Get(context.TODO(), name, c.Namespace(), &pod)
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := pod.ObjectMeta.Labels[key]; !ok || v != value {
			t.Errorf("expected pod %v to have label %v=%v, instead had: %v", name, key, value, v)
		}
		return ctx
	}
}
