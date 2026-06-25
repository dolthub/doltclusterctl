"""A test toolchain that can run on any execution platform.

Bazel 9's default test toolchain
(@bazel_tools//tools/test:default_test_toolchain) is declared with
use_target_platform_constraints, which requires the execution platform to
satisfy every constraint of the target platform. The incluster go_test is
cross-compiled to linux and only consumed as a data payload (loaded into a kind
cluster and run there), never executed by bazel, so on a non-linux host there is
no matching execution platform and toolchain resolution fails.

This toolchain returns an empty ToolchainInfo and, crucially, is NOT declared
with use_target_platform_constraints, so it matches any execution platform. We
scope it (via target_compatible_with on the toolchain() rule) to only the
platforms that carry the //:incluster_test marker constraint, so ordinary test
targets keep using Bazel's strict default toolchain.
"""

def _any_platform_test_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo()]

any_platform_test_toolchain = rule(
    implementation = _any_platform_test_toolchain_impl,
)
