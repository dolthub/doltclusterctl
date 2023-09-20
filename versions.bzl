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
        sha256 = "07d69502e58248927b58c7d7e7424135272ba5b2852a753ab6b67e62d2d29355",
        strip_prefix = "protobuf-24.3",
        urls = [
            "https://github.com/protocolbuffers/protobuf/releases/download/v24.3/protobuf-24.3.tar.gz",
        ],
    ),
    dolt_release_linux_amd64 = struct(
        sha256 = "2085c92afda3aac43cc6b92241f8c7e866c0f4c89bac03ff261d73d3c1a5c590",
        strip_prefix = "",
        urls = [
            "https://github.com/dolthub/dolt/releases/download/v1.16.1/dolt-linux-amd64.tar.gz",
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
        sha256 = "c71c25ed333a4909d2dd77e0b16c39e9912525a98c7fa85144282be8d04ef54c",
        strip_prefix = "rules_oci-1.3.4",
        urls = [
            "https://github.com/bazel-contrib/rules_oci/releases/download/v1.3.4/rules_oci-v1.3.4.tar.gz",
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
