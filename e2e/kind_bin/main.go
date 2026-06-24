package main

// A kludge to take a dependency on the kind binary so we can version what e2e
// uses with the bazel dependency closure instead of doing something like
// shelling out to `go install` in the e2e run.

import (
	// Import the same package the kind binary's main wraps, so `go mod tidy`
	// retains kind's full binary dependency closure (e.g. pkg/exec ->
	// github.com/alessio/shellescape). Importing only a leaf package like
	// pkg/fs prunes the rest, leaving gazelle without those repos.
	_ "sigs.k8s.io/kind/cmd/kind/app"

	_ "github.com/google/safetext/yamltemplate"
)

func main() {
}
