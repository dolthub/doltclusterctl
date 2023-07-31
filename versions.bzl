DEPS = struct(
    bazel_gazelle = struct(
        sha256 = "29218f8e0cebe583643cbf93cae6f971be8a2484cdcfa1e45057658df8d54002",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.32.0/bazel-gazelle-v0.32.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.32.0/bazel-gazelle-v0.32.0.tar.gz",
        ],
    ),
    com_github_bazelbuild_buildtools = struct(
        sha256 = "977a0bd4593c8d4c8f45e056d181c35e48aa01ad4f8090bdb84f78dca42f47dc",
        strip_prefix = "buildtools-6.1.2",
        urls = [
            "https://github.com/bazelbuild/buildtools/archive/refs/tags/v6.1.2.tar.gz",
        ],
    ),
    com_google_protobuf = struct(
        sha256 = "a700a49470d301f1190a487a923b5095bf60f08f4ae4cac9f5f7c36883d17971",
        strip_prefix = "protobuf-23.4",
        urls = [
            "https://github.com/protocolbuffers/protobuf/releases/download/v23.4/protobuf-23.4.tar.gz",
        ],
    ),
    dolt_release_linux_amd64 = struct(
        sha256 = "fa6e503706a492e43239930dc411b4e94220b4255151294f45181b7f86cac939",
        strip_prefix = "",
        urls = [
            "https://github.com/dolthub/dolt/releases/download/v1.8.7/dolt-linux-amd64.tar.gz",
        ],
    ),
    io_bazel_rules_go = struct(
        sha256 = "278b7ff5a826f3dc10f04feaf0b70d48b68748ccd512d7f98bf442077f043fe3",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.41.0/rules_go-v0.41.0.zip",
        ],
    ),
    rules_oci = struct(
        sha256 = "176e601d21d1151efd88b6b027a24e782493c5d623d8c6211c7767f306d655c8",
        strip_prefix = "rules_oci-1.2.0",
        urls = [
            "https://github.com/bazel-contrib/rules_oci/releases/download/v1.2.0/rules_oci-v1.2.0.tar.gz",
        ],
    ),
    rules_pkg = struct(
        sha256 = "8f9ee2dc10c1ae514ee599a8b42ed99fa262b757058f65ad3c384289ff70c4b8",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
            "https://github.com/bazelbuild/rules_pkg/releases/download/0.9.1/rules_pkg-0.9.1.tar.gz",
        ],
    ),
    toxiproxy_release_linux_amd64 = struct(
        sha256 = "2dcc53a7cd5e1cab3514bba3002cdc9626abd7e42cdf4a070242e3d977dcbeca",
        strip_prefix = "",
        urls = [
            "https://github.com/Shopify/toxiproxy/releases/download/v2.5.0/toxiproxy_2.5.0_linux_amd64.tar.gz",
        ],
    ),
)
