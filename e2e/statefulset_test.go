package main

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var StatefulSetKey *struct{}

func WithStatefulSet(ctx context.Context, statefulset *appsv1.StatefulSet) context.Context {
	return context.WithValue(ctx, &StatefulSetKey, statefulset)
}

func GetStatefulSet(ctx context.Context) (*appsv1.StatefulSet, bool) {
	if v := ctx.Value(&StatefulSetKey); v != nil {
		return v.(*appsv1.StatefulSet), true
	} else {
		return nil, false
	}
}

func DeleteStatefulSet(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	if statefulset, ok := GetStatefulSet(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Resources().Delete(ctx, statefulset); err != nil {
			t.Fatal(err)
		}
	}
	return ctx
}

func CreateStatefulSet(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	statefulset := NewStatefulSet(c.Namespace())
	client, err := c.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Resources().Create(ctx, statefulset); err != nil {
		t.Fatal(err)
	}
	err = wait.For(func() (bool, error) {
		if err := client.Resources().Get(context.TODO(), statefulset.GetName(), statefulset.GetNamespace(), statefulset); err != nil {
			return false, err
		}
		if statefulset.Status.ReadyReplicas == 2 && statefulset.Status.AvailableReplicas == 2 && statefulset.Status.CurrentReplicas == 2 {
			return true, nil
		}
		return false, nil
	}, wait.WithTimeout(time.Minute*1))
	if err != nil {
		t.Fatal(err)
	}

	return WithStatefulSet(ctx, statefulset)
}

func NewStatefulSet(namespace string) *appsv1.StatefulSet {
	labels := map[string]string{"app": "dolt"}
	var replicas int32 = 2
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:    &replicas,
			ServiceName: "dolt-internal",
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
	}
}
