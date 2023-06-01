package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const DoltImage = "docker.io/library/dolt:latest"
const DoltClusterCtlImage = "docker.io/library/doltclusterctl:latest"

// A very simple test which attempts to run the dolt image in the cluster.
func TestRunDoltSqlServer(t *testing.T) {
	deploymentName := "dolt"

	var deploymentKey string

	feature := features.New("Run dolt sql-server").
		WithSetup("create services", CreateServices).
		WithTeardown("delete services", DeleteServices).
		WithSetup("create test pod", CreateTestPod).
		WithTeardown("delete test pod", DeleteTestPod).
		WithSetup("create deployment", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			deployment := newDeployment(c.Namespace(), deploymentName, 1)
			client, err := c.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(deployment, appsv1.DeploymentAvailable, v1.ConditionTrue), wait.WithTimeout(time.Minute*5))
			if err != nil {
				t.Fatal(err)
			}

			return context.WithValue(ctx, &deploymentKey, deployment)
		}).
		Assess("TestConnectToService", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			client, err := c.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			command := []string{"/app/incluster_test", "-test.v", "-test.run", "TestConnectToService"}

			if err := client.Resources().ExecInPod(context.TODO(), c.Namespace(), InClusterPodName, "tail", command, &stdout, &stderr); err != nil {
				t.Log(stderr.String())
				t.Log(stdout.String())
				t.Fatal(err)
			}

			return ctx
		}).
		WithTeardown("delete deployment", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if v := ctx.Value(&deploymentKey); v != nil {
				deployment := v.(*appsv1.Deployment)
				client, err := c.NewClient()
				if err != nil {
					t.Fatal(err)
				}
				if err := client.Resources().Delete(ctx, deployment); err != nil {
					t.Fatal(err)
				}
			}
			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}

func newDeployment(namespace string, name string, replicas int32) *appsv1.Deployment {
	labels := map[string]string{"app": "dolt"}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:            "dolt",
						Image:           DoltImage,
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
						Image:           DoltImage,
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
