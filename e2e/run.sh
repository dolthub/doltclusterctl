#!/bin/bash

# A small wrapper to invoke e2e_test with its dependencies (the image tarballs
# and the kind binary) located through the Bazel runfiles library. Using the
# runfiles library lets us refer to dependencies by their apparent repository
# names instead of hardcoding canonical (bzlmod-mangled) runfiles paths.

set -eu

# --- begin runfiles.bash initialization v3 ---
set -uo pipefail
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
    source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
    source "$0.runfiles/$f" 2>/dev/null ||
    source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
    source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
    {
        echo >&2 "ERROR: cannot find $f"
        exit 1
    }
set -e
# --- end runfiles.bash initialization v3 ---

# Resolve a runfile, failing loudly if it can't be found. rlocation otherwise
# returns an empty string on a miss, which silently propagates as a bad arg.
function rloc() {
    local p
    p="$(rlocation "$1")"
    if [ -z "$p" ] || [ ! -e "$p" ]; then
        echo >&2 "ERROR: could not locate runfile: $1"
        exit 1
    fi
    echo "$p"
}

# The *.tar filegroups re-export the "tarball" output group of their oci_load
# targets, so the tarball files live under the oci_load target's directory.
export DOLTCLUSTERCTL_TAR="$(rloc com_github_dolthub_doltclusterctl/image_load/tarball.tar)"
export DOLT_TAR="$(rloc com_github_dolthub_doltclusterctl/e2e/dolt_loaded/tarball.tar)"
export DOLT_150_TAR="$(rloc com_github_dolthub_doltclusterctl/e2e/dolt_1_5_0_loaded/tarball.tar)"
export TOXIPROXY_TAR="$(rloc com_github_dolthub_doltclusterctl/e2e/toxiproxy_loaded/tarball.tar)"
export INCLUSTER_TAR="$(rloc com_github_dolthub_doltclusterctl/e2e/incluster/incluster_loaded/tarball.tar)"

# Put the kind binary on PATH so the e2e-framework uses it instead of trying to
# `go install` kind at runtime.
KIND_BIN="$(rloc io_k8s_sigs_kind/kind_/kind)"
export KIND_BIN
export PATH="$(dirname "${KIND_BIN}")":"${PATH}"

exec "$(rloc com_github_dolthub_doltclusterctl/e2e/e2e_test_/e2e_test)" "$@"
