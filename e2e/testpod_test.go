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
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const InClusterImage = "docker.io/library/incluster:latest"
const InClusterPodName = "incluster"
const InClusterContainerName = "incluster-test"

var TestPodKey *struct{}

func WithTestPod(ctx context.Context, pod *v1.Pod) context.Context {
	return context.WithValue(ctx, &TestPodKey, pod)
}

func GetTestPod(ctx context.Context) (*v1.Pod, bool) {
	if v := ctx.Value(&TestPodKey); v != nil {
		return v.(*v1.Pod), true
	} else {
		return nil, false
	}
}

func CreateTestPod(ctx context.Context, c *envconf.Config) (context.Context, error) {
	pod := NewTestPod(c.Namespace())
	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, pod); err != nil {
		return ctx, err
	}
	err = wait.For(conditions.New(client.Resources()).PodReady(pod), wait.WithTimeout(time.Minute*1))
	if err != nil {
		// Best effort cleanup
		_ = client.Resources().Delete(ctx, pod)
		return ctx, err
	}

	return WithTestPod(ctx, pod), nil
}

func DeleteTestPod(ctx context.Context, c *envconf.Config) (context.Context, error) {
	if pod, ok := GetTestPod(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			return ctx, err
		}
		if err := client.Resources().Delete(ctx, pod); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func NewTestPod(namespace string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: InClusterPodName, Namespace: namespace},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            InClusterContainerName,
				Image:           InClusterImage,
				ImagePullPolicy: v1.PullNever,
				Command:         []string{"/bin/tail", "-f", "/dev/null"},
			}},
		},
	}
}

type InClusterTest struct {
	TestName string
	DBName   string
}

func RunUnitTestInCluster(test InClusterTest) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		pod, ok := GetTestPod(ctx)
		if !ok {
			t.Fatal("did not find incluster test pod. must run in a step after CreateTestPod setup.")
		}

		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		command := []string{"/app/incluster_test", "-test.v", "-test.run", test.TestName}
		if test.DBName != "" {
			command = append(command, "-dbhostname", test.DBName)
		}
		if ss, ok := GetStatefulSet(ctx); ok {
			config := ss.Config
			if config.Username != "" {
				command = append(command, "-username", config.Username)
			}
			if config.Password != "" {
				command = append(command, "-password", config.Password)
			}
		}
		if err := client.Resources().ExecInPod(context.TODO(), pod.ObjectMeta.Namespace, pod.ObjectMeta.Name, pod.Spec.Containers[0].Name, command, &stdout, &stderr); err != nil {
			t.Log(stderr.String())
			t.Log(stdout.String())
			t.Errorf("error running %v in test pod: %v", command, err)
		}
		return ctx
	}
}
