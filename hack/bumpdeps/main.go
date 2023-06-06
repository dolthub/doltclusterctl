package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
)

var Client = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type ArchiveDependency struct {
	Name                 string
	LatestReleaseURL     string
	StripPrefix          string
	DownloadURLTemplates []string
}

var Deps = []ArchiveDependency{{
	Name:             "io_bazel_rules_go",
	LatestReleaseURL: "https://github.com/bazelbuild/rules_go/releases/latest",
	DownloadURLTemplates: []string{
		"https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/{{.Version}}/rules_go-{{.Version}}.zip",
		"https://github.com/bazelbuild/rules_go/releases/download/{{.Version}}/rules_go-{{.Version}}.zip",
	},
}, {
	Name:             "bazel_gazelle",
	LatestReleaseURL: "https://github.com/bazelbuild/bazel-gazelle/releases/latest",
	DownloadURLTemplates: []string{
		"https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/{{.Version}}/bazel-gazelle-{{.Version}}.tar.gz",
		"https://github.com/bazelbuild/bazel-gazelle/releases/download/{{.Version}}/bazel-gazelle-{{.Version}}.tar.gz",
	},
}, {
	Name:             "rules_pkg",
	LatestReleaseURL: "https://github.com/bazelbuild/rules_pkg/releases/latest",
	DownloadURLTemplates: []string{
		"https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/{{.Version}}/rules_pkg-{{.Version}}.tar.gz",
		"https://github.com/bazelbuild/rules_pkg/releases/download/{{.Version}}/rules_pkg-{{.Version}}.tar.gz",
	},
}, {
	Name:             "rules_oci",
	LatestReleaseURL: "https://github.com/bazel-contrib/rules_oci/releases/latest",
	StripPrefix:      "rules_oci-{{.VersionWithoutV}}",
	DownloadURLTemplates: []string{
		"https://github.com/bazel-contrib/rules_oci/releases/download/{{.Version}}/rules_oci-{{.Version}}.tar.gz",
	},
}, {
	Name:             "com_google_protobuf",
	LatestReleaseURL: "https://github.com/protocolbuffers/protobuf/releases/latest",
	StripPrefix:      "protobuf-{{.VersionWithoutV}}",
	DownloadURLTemplates: []string{
		"https://github.com/protocolbuffers/protobuf/releases/download/{{.Version}}/protobuf-{{.VersionWithoutV}}.tar.gz",
	},
}, {
	Name:             "com_github_bazelbuild_buildtools",
	LatestReleaseURL: "https://github.com/bazelbuild/buildtools/releases/latest",
	StripPrefix:      "buildtools-{{.VersionWithoutV}}",
	DownloadURLTemplates: []string{
		"https://github.com/bazelbuild/buildtools/archive/refs/tags/{{.Version}}.tar.gz",
	},
}, {
	Name:             "dolt_release_linux_amd64",
	LatestReleaseURL: "https://github.com/dolthub/dolt/releases/latest",
	DownloadURLTemplates: []string{
		"https://github.com/dolthub/dolt/releases/download/{{.Version}}/dolt-linux-amd64.tar.gz",
	},
}}

type ResolvedDependency struct {
	SHA256       string
	StripPrefix  string
	DownloadURLs []string
}

var ResolvedDependencies = make(map[string]ResolvedDependency)

func main() {
	err := ResolveDependencies()
	if err != nil {
		panic(err)
	}

	var b strings.Builder
	b.WriteString("DEPS = struct(\n")
	var keys []string
	for k := range ResolvedDependencies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := ResolvedDependencies[k]
		RenderDependency(&b, k, v)
	}
	b.WriteString(")\n")

	if len(os.Args) == 1 {
		fmt.Print(b.String())
	} else {
		f, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		_, err = fmt.Fprint(f, b.String())
		if err != nil {
			panic(err)
		}
	}
}

func ResolveDependencies() error {
	for _, d := range Deps {
		err := ResolveDependency(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func RenderDependency(b *strings.Builder, name string, d ResolvedDependency) {
	b.WriteString("    ")
	b.WriteString(name)
	b.WriteString(" = struct(\n")
	b.WriteString("        sha256 = \"")
	b.WriteString(d.SHA256)
	b.WriteString("\",\n")
	b.WriteString("        strip_prefix = \"")
	b.WriteString(d.StripPrefix)
	b.WriteString("\",\n")
	b.WriteString("        urls = [\n")
	for _, u := range d.DownloadURLs {
		b.WriteString("            \"")
		b.WriteString(u)
		b.WriteString("\",\n")
	}
	b.WriteString("        ],\n")
	b.WriteString("    ),\n")
}

func ResolveDependency(d ArchiveDependency) error {
	resp, err := Client.Head(d.LatestReleaseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	locURL, err := resp.Location()
	if err != nil {
		return err
	}
	loc := locURL.String()

	version := loc[strings.LastIndex(loc, "/")+1:]

	data := map[string]string{
		"Version":         version,
		"VersionWithoutV": strings.TrimPrefix(version, "v"),
	}

	var stripprefix string
	var downloadurls []string
	var sha256 string

	t := template.New("depstrings")
	var buf bytes.Buffer

	if d.StripPrefix != "" {
		_, err = t.Parse(d.StripPrefix)
		if err != nil {
			return err
		}
		err = t.Execute(&buf, data)
		if err != nil {
			return err
		}
		stripprefix = buf.String()
		buf.Reset()
	}

	for _, d := range d.DownloadURLTemplates {
		_, err = t.Parse(d)
		if err != nil {
			return err
		}
		err = t.Execute(&buf, data)
		if err != nil {
			return err
		}
		downloadurls = append(downloadurls, buf.String())
		buf.Reset()
	}

	sha256, err = GetSHA256(downloadurls)
	if err != nil {
		return err
	}

	ResolvedDependencies[d.Name] = ResolvedDependency{
		SHA256:       sha256,
		StripPrefix:  stripprefix,
		DownloadURLs: downloadurls,
	}

	return nil
}

func GetSHA256(urls []string) (string, error) {
	// We use the DefaultClient here in case we need to follow redirects.
	var lastErr error
	for _, u := range urls {
		resp, err := http.DefaultClient.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("non-200 response fetching %s: %d", u, resp.StatusCode)
			continue
		}

		shasum := sha256.New()
		_, err = io.Copy(shasum, resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		return hex.EncodeToString(shasum.Sum(nil)), nil
	}
	return "", lastErr
}
