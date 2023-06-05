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
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type TLSMode int

const (
	TLSModeNone     TLSMode = 0
	TLSModeOptional         = 1
	TLSModeRequired         = 2
)

type StatefulSetConfig struct {
	NumReplicas int32
	Username    string
	Password    string
	TLSMode     TLSMode
}

// Context state which represents the configuration and created resources for
// the StatefulSet.
type StatefulSetState struct {
	Config      StatefulSetConfig
	StatefulSet *appsv1.StatefulSet
	ConfigMaps  []*v1.ConfigMap
}

var StatefulSetStateKey *struct{}

func WithStatefulSetState(ctx context.Context, state StatefulSetState) context.Context {
	return context.WithValue(ctx, &StatefulSetStateKey, state)
}

func GetStatefulSet(ctx context.Context) (StatefulSetState, bool) {
	if v := ctx.Value(&StatefulSetStateKey); v != nil {
		return v.(StatefulSetState), true
	} else {
		return StatefulSetState{}, false
	}
}

func DeleteStatefulSet(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	if state, ok := GetStatefulSet(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		err = client.Resources().Delete(ctx, state.StatefulSet)
		if err != nil {
			t.Fatal(err)
		}
		for _, cm := range state.ConfigMaps {
			err = client.Resources().Delete(ctx, cm)
			if err != nil {
				t.Fatal(err)
			}
		}

		deadline := time.Now().Add(1 * time.Minute)

		err = wait.For(conditions.New(client.Resources()).ResourceDeleted(state.StatefulSet), wait.WithTimeout(deadline.Sub(time.Now())))
		if err != nil {
			t.Fatal(err)
		}
		for _, cm := range state.ConfigMaps {
			err = wait.For(conditions.New(client.Resources()).ResourceDeleted(cm), wait.WithTimeout(deadline.Sub(time.Now())))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return ctx
}

type StatefulSetOption func(*StatefulSetConfig)

func WithReplicas(count int32) StatefulSetOption {
	return func(config *StatefulSetConfig) {
		config.NumReplicas = count
	}
}

func WithCredentials(username, password string) StatefulSetOption {
	return func(config *StatefulSetConfig) {
		config.Username = username
		config.Password = password
	}
}

func WithTLSMode(mode TLSMode) StatefulSetOption {
	return func(config *StatefulSetConfig) {
		config.TLSMode = mode
	}
}

func CreateStatefulSet(opts ...StatefulSetOption) func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		var config StatefulSetConfig
		for _, o := range opts {
			o(&config)
		}
		statefulset, configmaps := NewStatefulSet(c.Namespace(), config)
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Resources().Create(ctx, statefulset); err != nil {
			t.Fatal(err)
		}
		for _, cm := range configmaps {
			if err := client.Resources().Create(ctx, cm); err != nil {
				t.Fatal(err)
			}
		}
		var numReplicas int32 = 1
		if statefulset.Spec.Replicas != nil {
			numReplicas = *statefulset.Spec.Replicas
		}
		err = wait.For(func() (bool, error) {
			if err := client.Resources().Get(context.TODO(), statefulset.GetName(), statefulset.GetNamespace(), statefulset); err != nil {
				return false, err
			}
			if statefulset.Status.AvailableReplicas == numReplicas {
				return true, nil
			}
			return false, nil
		}, wait.WithTimeout(time.Minute*1))
		if err != nil {
			t.Fatal(err)
		}

		return WithStatefulSetState(ctx, StatefulSetState{
			Config:      config,
			StatefulSet: statefulset,
			ConfigMaps:  configmaps,
		})
	}
}

func NewStatefulSet(namespace string, config StatefulSetConfig) (*appsv1.StatefulSet, []*v1.ConfigMap) {
	labels := map[string]string{"app": "dolt"}
	if config.NumReplicas == 0 {
		config.NumReplicas = 2
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:            &config.NumReplicas,
			ServiceName:         "dolt-internal",
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:            "dolt",
						Image:           DoltImage,
						ImagePullPolicy: v1.PullNever,
						Command:         []string{"/usr/local/bin/dolt", "sql-server", "--config", "config.yaml"},
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
						}, {
							Name:      "dolt-config",
							MountPath: "/etc/dolt",
						}},
					}},
					InitContainers: []v1.Container{{
						Name:            "init-dolt",
						Image:           DoltImage,
						ImagePullPolicy: v1.PullNever,
						Command: []string{"/bin/bash", "-c", `
dolt config --global --set metrics.disabled true
dolt config --global --set user.email testing-doltclusterctl@example.com
dolt config --global --set user.name "Testing doltcluster"
cp /etc/dolt/"${POD_NAME}".yaml /var/doltdb/config.yaml
`},
						WorkingDir: "/var/doltdb",
						Env: []v1.EnvVar{{
							Name:  "DOLT_ROOT_PATH",
							Value: "/var/doltdb",
						}, {
							Name: "POD_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
						}},
						VolumeMounts: []v1.VolumeMount{{
							Name:      "dolt-storage",
							MountPath: "/var/doltdb",
						}, {
							Name:      "dolt-config",
							MountPath: "/etc/dolt",
						}},
					}},
					Volumes: []v1.Volume{{
						Name: "dolt-storage",
						VolumeSource: v1.VolumeSource{
							EmptyDir: &v1.EmptyDirVolumeSource{},
						},
					}, {
						Name: "dolt-config",
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "dolt",
								},
							},
						},
					}},
				},
			},
		},
	}, SqlServerConfigMaps("dolt", namespace, config)
}

func TestSqlServerConfig(t *testing.T) {
	expected := `log_level: trace
cluster:
  standby_remotes:
  - name: dolt-1
    remote_url_template: http://dolt-1.dolt-internal:50051/{database}
  - name: dolt-2
    remote_url_template: http://dolt-2.dolt-internal:50051/{database}
  bootstrap_epoch: 1
  bootstrap_role: primary
  remotesapi:
    port: 50051
listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128
`
	got := SqlServerConfig(0, StatefulSetConfig{NumReplicas: 3})
	if got != expected {
		t.Errorf("SqlServerConfig(0, 3) should not have been\n%sblah\nexpected\n\n%sblah", got, expected)
	}
}

func StandbyRemoteStanza(num int32) string {
	return fmt.Sprintf(`  - name: dolt-%d
    remote_url_template: http://dolt-%d.dolt-internal:50051/{database}
`, num, num)
}

func ListenerStanza(config StatefulSetConfig) string {
	if config.TLSMode == TLSModeRequired {
		return `listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128
  tls_key: "/etc/dolt/key.pem"
  tls_cert: "/etc/dolt/chain.pem"
  required_secure_transport: true`
	} else if config.TLSMode == TLSModeOptional {
		return `listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128
  tls_key: "/etc/dolt/key.pem"
  tls_cert: "/etc/dolt/chain.pem"`
	} else {
		return `listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128`
	}
}

func SqlServerConfig(this int32, config StatefulSetConfig) string {
	var parts []string
	parts = append(parts, `log_level: trace
cluster:
  standby_remotes:
`)
	for i := int32(0); i < config.NumReplicas; i++ {
		if i != this {
			parts = append(parts, StandbyRemoteStanza(i))
		}
	}
	role := "standby"
	if this == 0 {
		role = "primary"
	}
	parts = append(parts, fmt.Sprintf(`  bootstrap_epoch: 1
  bootstrap_role: %s
  remotesapi:
    port: 50051
%s
`, role, ListenerStanza(config)))
	if config.Username != "" {
		parts = append(parts, fmt.Sprintf(`user:
  name: %s
`, config.Username))
		if config.Password != "" {
			parts = append(parts, fmt.Sprintf("  password: %s\n", config.Password))
		}
	}
	return strings.Join(parts, "")
}

func SqlServerConfigMaps(name, namespace string, config StatefulSetConfig) []*v1.ConfigMap {
	data := make(map[string]string)
	for i := int32(0); i < config.NumReplicas; i++ {
		data[fmt.Sprintf("dolt-%d.yaml", i)] = SqlServerConfig(i, config)
	}

	if config.TLSMode != TLSModeNone {
		bundle, err := NewTLSBundle(namespace, config)
		if err != nil {
			panic(err)
		}
		data["key.pem"] = bundle.Key
		data["chain.pem"] = bundle.Chain
	}

	serverConfig := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}
	return []*v1.ConfigMap{serverConfig}
}
