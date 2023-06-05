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

func TestRollingRestart(t *testing.T) {
	newcluster := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunRollingRestart", RunDoltClusterCtlJob(WithArgs("rollingrestart", "dolt"))).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-2/IsStandby", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	testenv.Test(t, newcluster)
}
