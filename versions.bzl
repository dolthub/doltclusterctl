DEPS = struct(
    bazel_gazelle = struct(
        sha256 = "b8b6d75de6e4bf7c41b7737b183523085f56283f6db929b86c5e7e1f09cf59c9",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.31.1/bazel-gazelle-v0.31.1.tar.gz",
            "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.31.1/bazel-gazelle-v0.31.1.tar.gz",
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
        sha256 = "3a5f47ad3aa10192c5577ff086b24b9739a36937c34ceab6db912a16a3ef7f8e",
        strip_prefix = "protobuf-23.3",
        urls = [
            "https://github.com/protocolbuffers/protobuf/releases/download/v23.3/protobuf-23.3.tar.gz",
        ],
    ),
    dolt_release_linux_amd64 = struct(
        sha256 = "c4c0fca3609b749e4827e198a79cd1168480fbda8a924259bfb6b4fa6573e6b6",
        strip_prefix = "",
        urls = [
            "https://github.com/dolthub/dolt/releases/download/v1.5.0/dolt-linux-amd64.tar.gz",
        ],
    ),
    io_bazel_rules_go = struct(
        sha256 = "6dc2da7ab4cf5d7bfc7c949776b1b7c733f05e56edc4bcd9022bb249d2e2a996",
        strip_prefix = "",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.39.1/rules_go-v0.39.1.zip",
        ],
    ),
    rules_oci = struct(
        sha256 = "db57efd706f01eb3ce771468366baa1614b5b25f4cce99757e2b8d942155b8ec",
        strip_prefix = "rules_oci-1.0.0",
        urls = [
            "https://github.com/bazel-contrib/rules_oci/releases/download/v1.0.0/rules_oci-v1.0.0.tar.gz",
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
