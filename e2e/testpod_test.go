package main

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const InClusterImage = "docker.io/library/incluster:latest"
const InClusterPodName = "incluster"

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

func CreateTestPod(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	pod := NewTestPod(c.Namespace())
	client, err := c.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Resources().Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	err = wait.For(conditions.New(client.Resources()).PodReady(pod), wait.WithTimeout(time.Minute*1))
	if err != nil {
		// Best effort cleanup
		_ = client.Resources().Delete(ctx, pod)
		t.Fatal(err)
	}

	return WithTestPod(ctx, pod)
}

func DeleteTestPod(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
	if pod, ok := GetTestPod(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Resources().Delete(ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	return ctx
}

func NewTestPod(namespace string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: InClusterPodName, Namespace: namespace},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            "tail",
				Image:           InClusterImage,
				ImagePullPolicy: v1.PullNever,
				Command:         []string{"/bin/tail", "-f", "/dev/null"},
			}},
		},
	}
}
