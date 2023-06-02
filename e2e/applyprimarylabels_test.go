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
		WithSetup("create statefulset", CreateStatefulSet).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob("applyprimarylabels", "dolt")).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-rw")).
		Assess("Connect/dolt-ro", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-ro")).
		Feature()
	testenv.Test(t, feature)
}

func RunDoltClusterCtlJob(args ...string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		name := envconf.RandomName("dcc-apl", 12)

		// Create the job.
		job := NewDoltClusterCtlJob(name, c.Namespace(), args...)

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

func NewDoltClusterCtlJob(name, namespace string, args ...string) *batchv1.Job {
	labels := map[string]string{"app": "doltclusterctl", "job": name}
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
