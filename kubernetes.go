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
	"fmt"
	"log"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const RoleLabel = "dolthub.com/cluster_role"
const PrimaryRoleValue = "primary"
const StandbyRoleValue = "standby"

// A Cluster implementation for a Kubernetes StatefulSet following certain
// conventions.
type kubernetesCluster struct {
	Namespace  string
	ObjectName string

	Clientset   *kubernetes.Clientset
	StatefulSet *appsv1.StatefulSet
	Pods        []*corev1.Pod
}

func NewKubernetesCluster(ctx context.Context, namespace, objectname string, clientset *kubernetes.Clientset) (Cluster, error) {
	cluster := &kubernetesCluster{
		Namespace:  namespace,
		ObjectName: objectname,
		Clientset:  clientset,
	}

	var err error
	cluster.StatefulSet, err = clientset.AppsV1().StatefulSets(namespace).Get(ctx, objectname, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error loading StatefulSet %s/%s: %w", namespace, objectname, err)
	}

	cluster.Pods = make([]*corev1.Pod, cluster.NumReplicas())
	for i := range cluster.Pods {
		podname := objectname + "-" + strconv.Itoa(i)
		cluster.Pods[i], err = clientset.CoreV1().Pods(namespace).Get(ctx, podname, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error loading Pod %s/%s for StatefulSet %s/%s: %w", namespace, podname, namespace, objectname, err)
		}
	}

	return cluster, nil
}

func (kc *kubernetesCluster) Name() string {
	// TODO: From the statefulset metadata.
	return fmt.Sprintf("%s/%s", kc.Namespace, kc.ObjectName)
}

func (kc *kubernetesCluster) ServiceName() string {
	return kc.StatefulSet.Spec.ServiceName
}

func (kc *kubernetesCluster) NumReplicas() int {
	if kc.StatefulSet.Spec.Replicas != nil {
		return int(*kc.StatefulSet.Spec.Replicas)
	}
	return 1
}

func (kc *kubernetesCluster) Instance(i int) Instance {
	return kubernetesClusterInstance{kc, i}
}

func (kc *kubernetesCluster) port() int {
	for _, c := range kc.StatefulSet.Spec.Template.Spec.Containers {
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

type kubernetesClusterInstance struct {
	cluster *kubernetesCluster
	replica int
}

func (i kubernetesClusterInstance) pod() *corev1.Pod {
	return i.cluster.Pods[i.replica]
}

func (i kubernetesClusterInstance) Port() int {
	return i.cluster.port()
}

func (i kubernetesClusterInstance) Name() string {
	p := i.pod()
	return p.Namespace + "/" + p.Name
}

func (i kubernetesClusterInstance) Hostname() string {
	p := i.pod()
	return p.Name + "." + i.cluster.ServiceName() + "." + p.Namespace
}

func (i kubernetesClusterInstance) MarkRolePrimary(ctx context.Context) error {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == PrimaryRoleValue {
		// Do not need to do anything...
		return nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = PrimaryRoleValue
		np, err := i.cluster.Clientset.CoreV1().Pods(i.cluster.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating pod %s to add dolthub.com/cluster_role=primary label: %w", i.Name(), err)
		}
		i.cluster.Pods[i.replica] = np
	}
	return nil
}

func (i kubernetesClusterInstance) MarkRoleStandby(ctx context.Context) error {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == StandbyRoleValue {
		// Do not need to do anything...
		return nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = StandbyRoleValue
		np, err := i.cluster.Clientset.CoreV1().Pods(i.cluster.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating pod %s to add dolthub.com/cluster_role=standby label: %w", i.Name(), err)
		}
		i.cluster.Pods[i.replica] = np
	}
	return nil
}

func (i kubernetesClusterInstance) MarkRoleUnknown(ctx context.Context) error {
	p := i.pod()
	if _, ok := p.ObjectMeta.Labels[RoleLabel]; !ok {
		// Do not need to do anything...
		return nil
	} else {
		delete(p.ObjectMeta.Labels, RoleLabel)
		np, err := i.cluster.Clientset.CoreV1().Pods(i.cluster.Namespace).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating pod %s to remove dolthub.com/cluster_role label: %w", i.Name(), err)
		}
		i.cluster.Pods[i.replica] = np
	}
	return nil
}

func (i kubernetesClusterInstance) Role() Role {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok {
		if v == StandbyRoleValue {
			return RoleStandby
		} else if v == PrimaryRoleValue {
			return RolePrimary
		}
	}
	return RoleUnknown
}

func (i kubernetesClusterInstance) Restart(ctx context.Context) error {
	p := i.pod()
	pods := i.cluster.Clientset.CoreV1().Pods(i.cluster.Namespace)
	w, err := pods.Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.Name).String(),
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
				log.Printf("poll for deletiong of pod %s finished with ctx.Done()", i.Name())
				return
			}
		}
	}()
	log.Printf("deleting pod %s", i.Name())
	err = pods.Delete(ctx, p.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	<-done
	log.Printf("pod %s successfully deleted", i.Name())

	pollInterval := 100 * time.Millisecond
PollPod:
	for {
		time.Sleep(pollInterval)
		if ctx.Err() != nil {
			return fmt.Errorf("error: pod %s did not become Ready after deleting it: %w", i.Name(), ctx.Err())
		}

		p, err := pods.Get(ctx, p.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		for _, c := range p.Status.ContainerStatuses {
			if !c.Ready {
				continue PollPod
			}
		}

		// If we get here, pod exists and all its containers are Ready.
		i.cluster.Pods[i.replica] = p
		return nil
	}
}
