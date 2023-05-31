package main

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

func TestMain(m *testing.M) {
	testenv := env.New()
	kindClusterName := envconf.RandomName("dolt-test-cluster", 16)
	namespace := envconf.RandomName("dolt-cluster", 16)

	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
	)

	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
