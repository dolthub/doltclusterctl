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
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	testenv         env.Environment
	kindClusterName string
	namespace       string
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName = envconf.RandomName("dolt-test-cluster", 16)
	namespace = envconf.RandomName("dolt-cluster", 16)

	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
		PatchCoreDNSConfigMap,
		CreateDoltClusterCtlServiceAccount,
		envfuncs.LoadImageArchiveToCluster(kindClusterName, os.Getenv("DOLTCLUSTERCTL_TAR")),
		envfuncs.LoadImageArchiveToCluster(kindClusterName, os.Getenv("DOLT_TAR")),
		envfuncs.LoadImageArchiveToCluster(kindClusterName, os.Getenv("INCLUSTER_TAR")),
		CreateServices,
		CreateTestPod,
	)

	testenv.Finish(
		DeleteTestPod,
		DeleteServices,
		envfuncs.DeleteNamespace(namespace),
	)

	os.Exit(testenv.Run(m))
}
