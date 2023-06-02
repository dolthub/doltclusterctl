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
	"strings"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// We do not want DNS caching while we run our tests.
func PatchCoreDNSConfigMap(ctx context.Context, c *envconf.Config) (context.Context, error) {
	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	var cm v1.ConfigMap
	err = client.Resources().Get(ctx, "coredns", "kube-system", &cm)
	if err != nil {
		return ctx, err
	}
	contents := cm.Data["Corefile"]
	contents = strings.ReplaceAll(contents, "    cache 30", "")
	cm.Data["Corefile"] = contents
	err = client.Resources().Update(ctx, &cm)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
