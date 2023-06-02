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
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestApplyPrimaryLabels(t *testing.T) {
	feature := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob("applyprimarylabels", "dolt")).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	password := features.New("WithPassword").
		WithSetup("create statefulset", CreateStatefulSet(WithCredentials(envconf.RandomName("user", 12), envconf.RandomName("pass", 12)))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob("applyprimarylabels", "dolt")).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	testenv.Test(t, feature, password)
}

func RunDoltClusterCtlJob(args ...string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		name := envconf.RandomName("doltclusterctl", 24)

		state, _ := GetStatefulSet(ctx)
		config := state.Config

		// Create the job.
		job := NewDoltClusterCtlJob(name, c.Namespace(), config, args...)

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
		err = wait.For(conditions.New(client.Resources()).JobCompleted(job), wait.WithTimeout(time.Minute*1))
		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func NewDoltClusterCtlJob(name, namespace string, config StatefulSetConfig, args ...string) *batchv1.Job {
	labels := map[string]string{"app": "doltclusterctl", "job": name}
	// In real life, these would ValueFrom a secret, but this is fine for the test, at least for now.
	var env []v1.EnvVar
	if config.Username != "" {
		env = append(env, v1.EnvVar{
			Name: "DOLT_USERNAME",
			Value: config.Username,
		})
	}
	if config.Password != "" {
		env = append(env, v1.EnvVar{
			Name: "DOLT_PASSWORD",
			Value: config.Password,
		})
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					ServiceAccountName: "doltclusterctl",
					Containers: []v1.Container{{
						Name:            "doltclusterctl",
						Image:           DoltClusterCtlImage,
						ImagePullPolicy: v1.PullNever,
						Command:         append([]string{"/usr/local/bin/doltclusterctl", "-n", namespace}, args...),
						Env:             env,
					}},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

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
