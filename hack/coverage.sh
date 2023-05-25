#!/bin/bash

set -eu

# Relies on lcov being installed in the environment...

mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$mydir"/..

outpath=$(bazel info output_path)

bazel coverage --combined_report=lcov //:coverage_tests
if [ -f "${outpath}"/_coverage/_coverage_report.dat ]; then
  lcov --list "${outpath}"/_coverage/_coverage_report.dat
  genhtml -o "${outpath}"/_coverage/genhtml $(bazel info output_path)/_coverage/_coverage_report.dat
  echo "${outpath}"/_coverage/genhtml/index.html
fi
