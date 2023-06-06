workspace(
    name = "com_github_dolthub_doltclusterctl",
)

load("@com_github_dolthub_doltclusterctl//:versions.bzl", "DEPS")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

####################
# Go lang
####################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = DEPS.io_bazel_rules_go.sha256,
    urls = DEPS.io_bazel_rules_go.urls,
)

http_archive(
    name = "bazel_gazelle",
    sha256 = DEPS.bazel_gazelle.sha256,
    urls = DEPS.bazel_gazelle.urls,
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("//:deps.bzl", "go_dependencies")

# gazelle:repository_macro deps.bzl%go_dependencies
go_dependencies()

go_rules_dependencies()

go_register_toolchains(
    nogo = "@//:nogo",
    version = "1.20.4",
)

gazelle_dependencies()

######################
# rules_pkg
######################

http_archive(
    name = "rules_pkg",
    sha256 = DEPS.rules_pkg.sha256,
    urls = DEPS.rules_pkg.urls,
)

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

######################
# rules_oci
######################

http_archive(
    name = "rules_oci",
    sha256 = DEPS.rules_oci.sha256,
    strip_prefix = DEPS.rules_oci.strip_prefix,
    urls = DEPS.rules_oci.urls,
)

load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")

rules_oci_dependencies()

load("@rules_oci//oci:repositories.bzl", "LATEST_CRANE_VERSION", "oci_register_toolchains")

oci_register_toolchains(
    name = "oci",
    crane_version = LATEST_CRANE_VERSION,
)

load(
    "@rules_oci//oci:pull.bzl",
    "oci_pull",
)

oci_pull(
    name = "ubuntu2004",
    # Find at https://l.gcr.io/google/ubuntu2004
    digest = "sha256:666805a878d9cd58ce9dfd6f423e2d0e3bdf01eea0cddd2be3a88244c1d4dfa5",
    image = "l.gcr.io/google/ubuntu2004",
)

######################
# Buildifier
######################

http_archive(
    name = "com_google_protobuf",
    sha256 = DEPS.com_google_protobuf.sha256,
    strip_prefix = DEPS.com_google_protobuf.strip_prefix,
    urls = DEPS.com_google_protobuf.urls,
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = DEPS.com_github_bazelbuild_buildtools.sha256,
    strip_prefix = DEPS.com_github_bazelbuild_buildtools.strip_prefix,
    urls = DEPS.com_github_bazelbuild_buildtools.urls,
)

#############################
# Dolt release for e2e tests
#############################

http_archive(
    name = "dolt_release_linux_amd64",
    build_file_content = """
exports_files(["bin/dolt"])
""",
    sha256 = DEPS.dolt_release_linux_amd64.sha256,
    strip_prefix = "dolt-linux-amd64",
    urls = DEPS.dolt_release_linux_amd64.urls,
)
