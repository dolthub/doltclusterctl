DEPS = struct(
    bazel_gazelle = struct(
        sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
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
        sha256 = "7beed9c511d632cff7c22ac0094dd7720e550153039d5da7e059bcceb488474a",
        strip_prefix = "protobuf-25.0",
        urls = [
            "https://github.com/protocolbuffers/protobuf/releases/download/v25.0/protobuf-25.0.tar.gz",
        ],
    ),
    dolt_release_linux_amd64 = struct(
        sha256 = "4a9b15c5fe93a99a95905edb6beb61c691957c763f1fe47556e68edb43835507",
        strip_prefix = "",
        urls = [
            "https://github.com/dolthub/dolt/releases/download/v1.43.13/dolt-linux-amd64.tar.gz",
        ],
    ),
    io_bazel_rules_go = struct(
        sha256 = "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
        ],
    ),
    rules_oci = struct(
        sha256 = "21a7d14f6ddfcb8ca7c5fc9ffa667c937ce4622c7d2b3e17aea1ffbc90c96bed",
        strip_prefix = "rules_oci-1.4.0",
        urls = [
            "https://github.com/bazel-contrib/rules_oci/releases/download/v1.4.0/rules_oci-v1.4.0.tar.gz",
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
        sha256 = "ea4ab0ff1e43a9880ce4f2094c43fe534b6610d1fea97406dcaf8fe7bc210bce",
        strip_prefix = "",
        urls = [
            "https://github.com/Shopify/toxiproxy/releases/download/v2.7.0/toxiproxy_2.7.0_linux_amd64.tar.gz",
        ],
    ),
)
