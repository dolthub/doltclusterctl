#!/bin/bash

set -eu

# Relies on lcov being installed in the environment...

mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$mydir"/..

bazel coverage --combined_report=lcov //:coverage_tests
lcov --list $(bazel info output_path)/_coverage/_coverage_report.dat
