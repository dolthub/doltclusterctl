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

func TestPromoteStandby(t *testing.T) {
	newcluster := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPromoteStandby", RunDoltClusterCtlJob(WithArgs("promotestandby", "dolt"))).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()

	// This test spins up a three node cluster, creates a database on the
	// primary, and replicates it to the standbys. It then breaks the
	// remotesapi port on the dolt-1 replica. It runs some more writes,
	// which will not replicate to dolt-1 successfully, and then it runs
	// `promotestandby`, asserting that dolt-2 is the replica which gets
	// promoted.
	mostrecent := features.New("MostRecent").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
		Assess("InsertReplicatedData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateWriteACKdDatabaseWithData", DBName: "dolt-rw"})).
		Assess("dolt-1/DisableRemotesAPI", RunUnitTestInCluster(InClusterTest{TestName: "TestDisableRemotesAPI", ToxiProxyEndpoint: "dolt-1.dolt-internal:8474"})).
		Assess("InsertMoreData", RunUnitTestInCluster(InClusterTest{TestName: "TestInsertMoreACKdData", DBName: "dolt-rw"})).
		Assess("RunPromoteStandby", RunDoltClusterCtlJob(WithArgs("promotestandby", "dolt"))).
		Assess("dolt-2/IsPrimary", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("AllDataIsPresent", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertWriteACKdDataPresent", DBName: "dolt-rw"})).
		Feature()

	testenv.Test(t, newcluster, mostrecent)
}
