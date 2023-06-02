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
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"
)

const DoltClusterCtlImage = "docker.io/library/doltclusterctl:latest"

// A very simple test which attempts to run the dolt image in the cluster.
func TestRunDoltDeployment(t *testing.T) {
	feature := features.New("DoltDeployment").
		WithSetup("create deployment", CreateDeployment).
		WithTeardown("delete deployment", DeleteDeployment).
		Assess("TestConnectToService", RunUnitTestInCluster("-test.run", "TestConnectToService")).
		Feature()
	testenv.Test(t, feature)
}

func TestRunDoltStatefulSet(t *testing.T) {
	feature := features.New("DoltStatefulSet").
		WithSetup("create statefulset", CreateStatefulSet).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("TestConnectToService", RunUnitTestInCluster("-test.run", "TestConnectToService")).
		Assess("TestConnectToService/dolt-0", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-0.dolt-internal")).
		Assess("TestConnectToService/dolt-1", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-1.dolt-internal")).
		Feature()
	testenv.Test(t, feature)
}
