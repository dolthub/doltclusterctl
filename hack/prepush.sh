#!/bin/bash

set -eu

# A quick and dirty script to apply some hygiene to the repo.

mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$mydir"/..

bazel run @go_sdk//:bin/go -- mod tidy
bazel run //:gazelle
bazel run //:gazelle-update-repos

bazel run //:buildifier

bazel run //hack:gofmt
