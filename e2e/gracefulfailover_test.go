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

func TestGracefulFailover(t *testing.T) {
	newcluster := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	cycles := features.New("Cycles").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	counts := features.New("CountsUp").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("dolt-2/IsPrimary", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	preservesdata := features.New("PreservesData").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob("applyprimarylabels", "dolt")).
		Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob("gracefulfailover", "dolt")).
		Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
		Feature()
	testenv.Test(t, newcluster, cycles, counts, preservesdata)
}
