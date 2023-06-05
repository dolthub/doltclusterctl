workspace(
    name = "com_github_dolthub_doltclusterctl",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

####################
# Go lang
####################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6dc2da7ab4cf5d7bfc7c949776b1b7c733f05e56edc4bcd9022bb249d2e2a996",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "727f3e4edd96ea20c29e8c2ca9e8d2af724d8c7778e7923a854b2c80952bc405",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.30.0/bazel-gazelle-v0.30.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.30.0/bazel-gazelle-v0.30.0.tar.gz",
    ],
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
    sha256 = "8f9ee2dc10c1ae514ee599a8b42ed99fa262b757058f65ad3c384289ff70c4b8",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
    ],
)

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

######################
# rules_oci
######################

http_archive(
    name = "rules_oci",
    sha256 = "db57efd706f01eb3ce771468366baa1614b5b25f4cce99757e2b8d942155b8ec",
    strip_prefix = "rules_oci-1.0.0",
    url = "https://github.com/bazel-contrib/rules_oci/releases/download/v1.0.0/rules_oci-v1.0.0.tar.gz",
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
    digest = "sha256:27a5b5335a18890fd424e71878a86124d930284ac962dc167ff7f8676e78a573",
    image = "l.gcr.io/google/ubuntu2004",
)

######################
# Buildifier
######################

http_archive(
    name = "com_google_protobuf",
    sha256 = "ddf8c9c1ffccb7e80afd183b3bd32b3b62f7cc54b106be190bf49f2bc09daab5",
    strip_prefix = "protobuf-23.2",
    urls = [
        "https://github.com/protocolbuffers/protobuf/releases/download/v23.2/protobuf-23.2.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "977a0bd4593c8d4c8f45e056d181c35e48aa01ad4f8090bdb84f78dca42f47dc",
    strip_prefix = "buildtools-6.1.2",
    urls = [
        "https://github.com/bazelbuild/buildtools/archive/refs/tags/v6.1.2.tar.gz",
    ],
)

#############################
# Dolt release for e2e tests
#############################

http_archive(
    name = "dolt_release_linux_amd64",
    build_file_content = """
exports_files(["bin/dolt"])
""",
    sha256 = "218b92ca12b5785d37796311c16315aa37bf960064bb8ffa3315864273e59fa7",
    strip_prefix = "dolt-linux-amd64",
    urls = [
        "https://github.com/dolthub/dolt/releases/download/v1.2.1/dolt-linux-amd64.tar.gz",
    ],
)
