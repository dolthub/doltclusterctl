package main

// A kludge to take a dependency on the kind binary so we can version what e2e
// uses with the bazel dependency closure instead of doing something like
// shelling out to `go install` in the e2e run.

import (
	_ "sigs.k8s.io/kind/pkg/fs"
	_ "github.com/google/safetext/yamltemplate"
)

func main() {
}
