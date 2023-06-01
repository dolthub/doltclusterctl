package main

import (
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/features"
)

const DoltClusterCtlImage = "docker.io/library/doltclusterctl:latest"

// A very simple test which attempts to run the dolt image in the cluster.
func TestRunDoltSqlServer(t *testing.T) {
	feature := features.New("Run dolt sql-server").
		WithSetup("create services", CreateServices).
		WithTeardown("delete services", DeleteServices).
		WithSetup("create test pod", CreateTestPod).
		WithTeardown("delete test pod", DeleteTestPod).
		WithSetup("create deployment", CreateDeployment).
		WithTeardown("delete deployment", DeleteDeployment).
		Assess("TestConnectToService", RunUnitTestInCluster("-test.run", "TestConnectToService")).
		Feature()
	testenv.Test(t, feature)
}
