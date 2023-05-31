#!/bin/bash

set -eu

mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$mydir"/..

outpath=$(bazel info output_path)

bazel run //e2e:run_e2e
