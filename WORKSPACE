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
# Docker
######################

# Docker support

# xz binary for rules_docker

http_archive(
    name = "rules_foreign_cc",
    sha256 = "076b8217296ca25d5b2167a832c8703cc51cbf8d980f00d6c71e9691876f6b08",
    strip_prefix = "rules_foreign_cc-2c6262f8f487cd3481db27e2c509d9e6d30bfe53",
    url = "https://github.com/bazelbuild/rules_foreign_cc/archive/2c6262f8f487cd3481db27e2c509d9e6d30bfe53.tar.gz",
)

load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")

rules_foreign_cc_dependencies()

http_archive(
    name = "xz-build",
    build_file_content = """
load("@rules_foreign_cc//foreign_cc:defs.bzl", "configure_make", "runnable_binary")

filegroup(
    name = "all_srcs",
    srcs = glob(["**"]),
)

configure_make(
    name = "xz-build",
    lib_source = ":all_srcs",
    out_binaries = ["xz"],
    env = {
        "AR": "",
    },
    configure_options = ["--enable-threads=no"],
)

runnable_binary(
    name = "xz",
    binary = "xz",
    foreign_cc_target = "@xz-build//:xz-build",
    visibility = ["//visibility:public"],
)
""",
    sha256 = "b194507fba3a462a753c553149ccdaa168337bcb7deefddd067ba987c83dfce6",
    strip_prefix = "xz-5.2.9",
    urls = [
        "https://tukaani.org/xz/xz-5.2.9.tar.bz2",
    ],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz"],
)

load(
    "@io_bazel_rules_docker//toolchains/docker:toolchain.bzl",
    docker_toolchain_configure = "toolchain_configure",
)

docker_toolchain_configure(
    name = "docker_config",
    xz_target = "@xz-build//:xz",
)

load("@io_bazel_rules_docker//repositories:repositories.bzl", container_repositories = "repositories")

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_pull(
    name = "ubuntu2004",
    # Find at https://l.gcr.io/google/ubuntu2004
    digest = "sha256:27a5b5335a18890fd424e71878a86124d930284ac962dc167ff7f8676e78a573",
    registry = "l.gcr.io",
    repository = "google/ubuntu2004",
    # tag field is ignored since digest is set
    tag = "latest",
)

######################
# Buildifier
######################

http_archive(
    name = "com_google_protobuf",
    sha256 = "3bd7828aa5af4b13b99c191e8b1e884ebfa9ad371b0ce264605d347f135d2568",
    strip_prefix = "protobuf-3.19.4",
    urls = [
        "https://github.com/protocolbuffers/protobuf/archive/v3.19.4.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "ae34c344514e08c23e90da0e2d6cb700fcd28e80c02e23e4d5715dddcb42f7b3",
    strip_prefix = "buildtools-4.2.2",
    urls = [
        "https://github.com/bazelbuild/buildtools/archive/refs/tags/4.2.2.tar.gz",
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
