package main

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestPromoteStandby(t *testing.T) {
	feature := features.New("NewCluster").
		WithSetup("create statefulset", CreateStatefulSet).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("RunPromoteStandby", RunDoltClusterCtlJob("promotestandby", "dolt")).
		Assess("dolt-1/IsPrimary", AssertPodHasLabel("dolt-1", "dolthub.com/cluster_role", "primary")).
		Assess("dolt-0/IsStandby", AssertPodHasLabel("dolt-0", "dolthub.com/cluster_role", "standby")).
		Assess("Connect/dolt-rw", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-rw")).
		Assess("Connect/dolt-ro", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-ro")).
		Feature()
	testenv.Test(t, feature)
}
