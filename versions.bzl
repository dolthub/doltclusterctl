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
        sha256 = "42968f9134ba2c75c03bb271bd7bb062afb7da449f9b913c96e5be4ce890030a",
        strip_prefix = "buildtools-6.3.3",
        urls = [
            "https://github.com/bazelbuild/buildtools/archive/refs/tags/v6.3.3.tar.gz",
        ],
    ),
    com_google_protobuf = struct(
        sha256 = "39b52572da90ad54c883a828cb2ca68e5ac918aa75d36c3e55c9c76b94f0a4f7",
        strip_prefix = "protobuf-24.2",
        urls = [
            "https://github.com/protocolbuffers/protobuf/releases/download/v24.2/protobuf-24.2.tar.gz",
        ],
    ),
    dolt_release_linux_amd64 = struct(
        sha256 = "1cd1d012c032c93ca8e65b77fa666d39a03f026614d0960d702d46a0840da5b9",
        strip_prefix = "",
        urls = [
            "https://github.com/dolthub/dolt/releases/download/v1.13.6/dolt-linux-amd64.tar.gz",
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
        sha256 = "fc8551ccbfe4e716c8a3876b1b42d37e80f0bbd5045ec4de3bed88f0dc1ff0aa",
        strip_prefix = "rules_oci-1.3.2",
        urls = [
            "https://github.com/bazel-contrib/rules_oci/releases/download/v1.3.2/rules_oci-v1.3.2.tar.gz",
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
        sha256 = "f3514328c66dcccba41ee175639ff96efe7e623515b54e2f82c06578e05a1337",
        strip_prefix = "",
        urls = [
            "https://github.com/Shopify/toxiproxy/releases/download/v2.6.0/toxiproxy_2.6.0_linux_amd64.tar.gz",
        ],
    ),
)
