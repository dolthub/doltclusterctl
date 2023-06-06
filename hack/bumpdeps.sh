#!/bin/bash

set -eu

mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$mydir"/..

trap "rm -f versions.bzl.new" EXIT
bazel run //hack/bumpdeps -- `pwd`/versions.bzl.new
mv versions.bzl.new versions.bzl
