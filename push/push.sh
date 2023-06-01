#!/bin/bash

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

export PATH="${RUNFILES}/com_github_awslabs_amazon_ecr_credential_helper_ecr_login/cli/docker-credential-ecr-login/docker-credential-ecr-login_":"${PATH}"
export AWS_SDK_LOAD_CONFIG=1
export AWS_CONFIG_FILE="${RUNFILES}/com_github_dolthub_doltclusterctl/push/awsconfig"
export AWS_PROFILE=imagewriter
export DOCKER_CONFIG="${RUNFILES}/com_github_dolthub_doltclusterctl/push/"

cd "${RUNFILES}/com_github_dolthub_doltclusterctl"

exec "${RUNFILES}/com_github_dolthub_doltclusterctl/push/push__push_public_image.sh"
