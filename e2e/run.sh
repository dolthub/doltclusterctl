#!/bin/bash

# A small wrapper to invoke e2e_test with the Golang SDK and dependencies from
# runfiles.

set -eu

function guess_runfiles() {
    if [ -d ${BASH_SOURCE[0]}.runfiles ]; then
        # Runfiles are adjacent to the current script.
        echo "$( cd ${BASH_SOURCE[0]}.runfiles && pwd )"
    else
        # The current script is within some other script's runfiles.
        mydir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
        echo $mydir | sed -e 's|\(.*\.runfiles\)/.*|\1|'
    fi
}

RUNFILES="${PYTHON_RUNFILES:-$(guess_runfiles)}"

export PATH="${RUNFILES}/go_sdk/bin":"${PATH}"

exec "${RUNFILES}"/com_github_dolthub_doltclusterctl/e2e/e2e_test_/e2e_test "$@"
