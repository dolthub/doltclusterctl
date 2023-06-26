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
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	cycles := features.New("Cycles").
		WithSetup("create statefulset", CreateStatefulSet()).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	counts := features.New("CountsUp").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("dolt-2/IsPrimary", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-rw"})).
		Assess("Connect/dolt-ro", RunUnitTestInCluster(InClusterTest{TestName: "TestConnectToService", DBName: "dolt-ro"})).
		Feature()
	preservesdata := features.New("PreservesData").
		WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
		Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
		Assess("RunGracefulFailover", RunDoltClusterCtlJob(WithArgs("gracefulfailover", "dolt"))).
		Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
		Feature()
	testenv.Test(t, newcluster, cycles, counts, preservesdata)

	t.Run("MinCaughtupStandbys", func(t *testing.T) {
		// -min-caughtup-standbys fails early against 1.5.0
		against150 := features.New("Against1.5.0").
			WithSetup("create statefulset", CreateStatefulSet(WithImageTag("v1.5.0"))).
			WithTeardown("delete statefulset", DeleteStatefulSet).
			Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
			Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
			Assess("RunGracefulFailover", RunDoltClusterCtlJob(
				WithArgs("-min-caughtup-standbys", "1", "gracefulfailover", "dolt"),
				ShouldFailWith(" does not support dolt_cluster_transition_to_standby."))).
			Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
			Feature()

		// -min-caughtup-standbys fails with count too high &mdash; 3 on 3 node cluster, 4 on 3 node cluster.
		minTooHigh := features.New("MinTooHigh").
			WithSetup("create statefulset", CreateStatefulSet()).
			WithTeardown("delete statefulset", DeleteStatefulSet).
			Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
			Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
			Assess("RunGracefulFailover", RunDoltClusterCtlJob(
				WithArgs("-min-caughtup-standbys", "2", "gracefulfailover", "dolt"),
				ShouldFailWith("Only 2 pods are in the cluster, so only 1 standbys can ever be caught up."))).
			Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
			Feature()

		// TODO: -min-caughtup-standbys fails with count above available replicas &mdash; 2 on 3 node cluster with on disabled
		// This will require toxiproxy on MySQL port as well.

		// -min-caughtup-standbys succeeds on 3 node cluster, min-caught-up of 1, one node is disabled.
		worksOneOutOfThree := features.New("WorksOneOutOfThree").
			WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
			WithTeardown("delete statefulset", DeleteStatefulSet).
			Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
			Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
			Assess("dolt-1/DisableRemotesAPI", RunUnitTestInCluster(InClusterTest{TestName: "TestDisableRemotesAPI", ToxiProxyEndpoint: "dolt-1.dolt-internal:8474"})).
			Assess("RunGracefulFailover", RunDoltClusterCtlJob(
				WithArgs("-min-caughtup-standbys", "1", "gracefulfailover", "dolt"))).
			Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
			Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
			Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
			Assess("dolt-2/IsPrimary", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "primary")).
			Feature()

		// -min-caughtup-standbys fails on 3 node cluster, min-caught-up of 2, one node is disabled.
		failsTwoOutOfThree := features.New("FailsTwoOutOfThree").
			WithSetup("create statefulset", CreateStatefulSet(WithReplicas(3))).
			WithTeardown("delete statefulset", DeleteStatefulSet).
			Assess("RunPrimaryLabels", RunDoltClusterCtlJob(WithArgs("applyprimarylabels", "dolt"))).
			Assess("CreateData", RunUnitTestInCluster(InClusterTest{TestName: "TestCreateSomeData", DBName: "dolt-rw"})).
			Assess("dolt-1/DisableRemotesAPI", RunUnitTestInCluster(InClusterTest{TestName: "TestDisableRemotesAPI", ToxiProxyEndpoint: "dolt-1.dolt-internal:8474"})).
			Assess("RunGracefulFailover", RunDoltClusterCtlJob(
				WithArgs("-min-caughtup-standbys", "2", "gracefulfailover", "dolt"),
				ShouldFailWith("failed to transition primary to standby. labeling old primary as primary."))).
			Assess("AssertData", RunUnitTestInCluster(InClusterTest{TestName: "TestAssertCreatedDataPresent", DBName: "dolt-rw"})).
			Assess("dolt-0/IsPrimary", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "primary")).
			Assess("dolt-1/IsStandby", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "standby")).
			Assess("dolt-2/IsStandby", AssertPodHasLabel("dolt-2", "dolthub.com/cluster_role", "standby")).
			Feature()

		testenv.Test(t, against150, minTooHigh, worksOneOutOfThree, failsTwoOutOfThree)
	})
}
