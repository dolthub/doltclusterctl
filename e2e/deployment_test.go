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

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const DoltImage = "docker.io/library/dolt"

var DeploymentKey *struct{}

func WithDeployment(ctx context.Context, deployment *appsv1.Deployment) context.Context {
	return context.WithValue(ctx, &DeploymentKey, deployment)
}

func GetDeployment(ctx context.Context) (*appsv1.Deployment, bool) {
	if v := ctx.Value(&DeploymentKey); v != nil {
		return v.(*appsv1.Deployment), true
	} else {
		return nil, false
	}
}

func DeleteDeployment(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	if deployment, ok := GetDeployment(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		err = client.Resources().Delete(ctx, deployment)
		if err != nil {
			t.Fatal(err)
		}
		err = wait.For(conditions.New(client.Resources()).ResourceDeleted(deployment), wait.WithTimeout(time.Minute*1))
		if err != nil {
			t.Fatal(err)
		}
	}
	return ctx
}

func CreateDeployment(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	deployment := NewDeployment(c.Namespace())
	client, err := c.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Resources().Create(ctx, deployment); err != nil {
		t.Fatal(err)
	}
	err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(time.Minute*1))
	if err != nil {
		t.Fatal(err)
	}

	return WithDeployment(ctx, deployment)
}

func NewDeployment(namespace string) *appsv1.Deployment {
	labels := map[string]string{"app": "dolt"}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:            "dolt",
						Image:           DoltImage + ":latest",
						ImagePullPolicy: v1.PullNever,
						Command:         []string{"/usr/local/bin/dolt", "sql-server", "-H", "0.0.0.0"},
						Ports: []v1.ContainerPort{{
							Name:          "dolt",
							ContainerPort: 3306,
						}},
						WorkingDir: "/var/doltdb",
						Env: []v1.EnvVar{{
							Name:  "DOLT_ROOT_PATH",
							Value: "/var/doltdb",
						}},
						VolumeMounts: []v1.VolumeMount{{
							Name:      "dolt-storage",
							MountPath: "/var/doltdb",
						}},
					}},
					InitContainers: []v1.Container{{
						Name:            "init-dolt",
						Image:           DoltImage + ":latest",
						ImagePullPolicy: v1.PullNever,
						Command: []string{"/bin/bash", "-c", `
dolt config --global --set metrics.disabled true
dolt config --global --set user.email testing-doltclusterctl@example.com
dolt config --global --set user.name "Testing doltcluster"
`},
						WorkingDir: "/var/doltdb",
						Env: []v1.EnvVar{{
							Name:  "DOLT_ROOT_PATH",
							Value: "/var/doltdb",
						}},
						VolumeMounts: []v1.VolumeMount{{
							Name:      "dolt-storage",
							MountPath: "/var/doltdb",
						}},
					}},
					Volumes: []v1.Volume{{
						Name: "dolt-storage",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					}},
				},
			},
		},
	}
}
