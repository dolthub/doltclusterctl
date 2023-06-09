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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var ServiceKey *struct{}

type Services struct {
	AllPods   *v1.Service
	ReadOnly  *v1.Service
	ReadWrite *v1.Service
	Internal  *v1.Service
}

func WithServices(ctx context.Context, svcs Services) context.Context {
	return context.WithValue(ctx, &ServiceKey, svcs)
}

func GetServices(ctx context.Context) (Services, bool) {
	if v := ctx.Value(&ServiceKey); v != nil {
		return v.(Services), true
	} else {
		return Services{}, false
	}
}

func DeleteServices(ctx context.Context, c *envconf.Config) (context.Context, error) {
	if svcs, ok := GetServices(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			return ctx, err
		}
		err = client.Resources().Delete(ctx, svcs.AllPods)
		if ferr := client.Resources().Delete(ctx, svcs.ReadOnly); ferr != nil {
			err = ferr
		}
		if ferr := client.Resources().Delete(ctx, svcs.ReadWrite); ferr != nil {
			err = ferr
		}
		if ferr := client.Resources().Delete(ctx, svcs.Internal); ferr != nil {
			err = ferr
		}
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func CreateServices(ctx context.Context, c *envconf.Config) (context.Context, error) {
	var svcs Services
	svcs.AllPods = NewDoltService(c.Namespace())
	svcs.ReadOnly = NewDoltROService(c.Namespace())
	svcs.ReadWrite = NewDoltRWService(c.Namespace())
	svcs.Internal = NewDoltInternalService(c.Namespace())

	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, svcs.AllPods); err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, svcs.ReadOnly); err != nil {
		// Best effort cleanup
		_ = client.Resources().Delete(ctx, svcs.AllPods)
		return ctx, err
	}
	if err := client.Resources().Create(ctx, svcs.ReadWrite); err != nil {
		// Best effort cleanup
		_ = client.Resources().Delete(ctx, svcs.AllPods)
		_ = client.Resources().Delete(ctx, svcs.ReadOnly)
		return ctx, err
	}
	if err := client.Resources().Create(ctx, svcs.Internal); err != nil {
		// Best effort cleanup
		_ = client.Resources().Delete(ctx, svcs.AllPods)
		_ = client.Resources().Delete(ctx, svcs.ReadOnly)
		_ = client.Resources().Delete(ctx, svcs.ReadWrite)
		return ctx, err
	}
	return WithServices(ctx, svcs), nil
}

func NewDoltROService(namespace string) *v1.Service {
	labels := map[string]string{"app": "dolt", "dolthub.com/cluster_role": "standby"}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt-ro", Namespace: namespace},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Name:       "dolt",
				Port:       3306,
				TargetPort: intstr.FromInt(3306),
			}},
		},
	}
}

func NewDoltRWService(namespace string) *v1.Service {
	labels := map[string]string{"app": "dolt", "dolthub.com/cluster_role": "primary"}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt-rw", Namespace: namespace},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Name:       "dolt",
				Port:       3306,
				TargetPort: intstr.FromInt(3306),
			}},
		},
	}
}

func NewDoltService(namespace string) *v1.Service {
	labels := map[string]string{"app": "dolt"}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt", Namespace: namespace},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Name:       "dolt",
				Port:       3306,
				TargetPort: intstr.FromInt(3306),
			}},
		},
	}
}

func NewDoltInternalService(namespace string) *v1.Service {
	labels := map[string]string{"app": "dolt"}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt-internal", Namespace: namespace},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Name:       "dolt",
				Port:       3306,
				TargetPort: intstr.FromInt(3306),
			}},
			ClusterIP: "None",
		},
	}
}
