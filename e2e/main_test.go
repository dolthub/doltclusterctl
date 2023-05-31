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
		envfuncs.LoadImageArchiveToCluster(kindClusterName, os.Getenv("DOLTCLUSTERCTL_TAR")),
		envfuncs.LoadImageArchiveToCluster(kindClusterName, os.Getenv("DOLT_TAR")),
	)

	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
	)

	os.Exit(testenv.Run(m))
}
