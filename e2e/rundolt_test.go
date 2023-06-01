package main

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"
)

const DoltClusterCtlImage = "docker.io/library/doltclusterctl:latest"

// A very simple test which attempts to run the dolt image in the cluster.
func TestRunDoltDeployment(t *testing.T) {
	feature := features.New("Run dolt sql-server").
		WithSetup("create deployment", CreateDeployment).
		WithTeardown("delete deployment", DeleteDeployment).
		Assess("TestConnectToService", RunUnitTestInCluster("-test.run", "TestConnectToService")).
		Feature()
	testenv.Test(t, feature)
}

func TestRunDoltStatefulSet(t *testing.T) {
	feature := features.New("Run dolt sql-server").
		WithSetup("create statefulset", CreateStatefulSet).
		WithTeardown("delete statefulset", DeleteStatefulSet).
		Assess("TestConnectToService", RunUnitTestInCluster("-test.run", "TestConnectToService")).
		Assess("TestConnectToService/dolt-0", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-0.dolt-internal")).
		Assess("TestConnectToService/dolt-1", RunUnitTestInCluster("-test.run", "TestConnectToService", "-dbhostname", "dolt-1.dolt-internal")).
		Feature()
	testenv.Test(t, feature)
}
