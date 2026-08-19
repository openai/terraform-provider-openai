package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-version"
)

func TestInstallTerraformRejectsSubstitutedReleaseMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*releaseMetadata)
	}{
		{
			name: "older signed release",
			mutate: func(metadata *releaseMetadata) {
				metadata.Version = "1.15.7"
				metadata.Checksums = "terraform_1.15.7_SHA256SUMS"
				metadata.Signature = "terraform_1.15.7_SHA256SUMS.sig"
				metadata.Signatures = []string{"terraform_1.15.7_SHA256SUMS.sig"}
				build := &metadata.Builds[0]
				build.Version = "1.15.7"
				build.Filename = strings.ReplaceAll(build.Filename, "1.15.8", "1.15.7")
				build.URL = strings.ReplaceAll(build.URL, "1.15.8", "1.15.7")
			},
		},
		{
			name: "different product",
			mutate: func(metadata *releaseMetadata) {
				metadata.Name = "vault"
			},
		},
		{
			name: "different signed manifest",
			mutate: func(metadata *releaseMetadata) {
				metadata.Checksums = "terraform_1.15.7_SHA256SUMS"
			},
		},
		{
			name: "different signed manifest signature",
			mutate: func(metadata *releaseMetadata) {
				metadata.Signatures = []string{"terraform_1.15.7_SHA256SUMS.sig"}
			},
		},
		{
			name: "different build filename",
			mutate: func(metadata *releaseMetadata) {
				build := &metadata.Builds[0]
				build.Filename = strings.ReplaceAll(build.Filename, "1.15.8", "1.15.7")
			},
		},
		{
			name: "different build version",
			mutate: func(metadata *releaseMetadata) {
				metadata.Builds[0].Version = "1.15.7"
			},
		},
		{
			name: "different build product",
			mutate: func(metadata *releaseMetadata) {
				metadata.Builds[0].Name = "vault"
			},
		},
		{
			name: "different build URL",
			mutate: func(metadata *releaseMetadata) {
				metadata.Builds[0].URL = "https://example.com/terraform.zip"
			},
		},
		{
			name: "missing platform build",
			mutate: func(metadata *releaseMetadata) {
				metadata.Builds[0].Arch = "unsupported"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			metadata := terraformReleaseMetadata("1.15.8")
			test.mutate(&metadata)

			var artifactRequests atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/terraform/1.15.8/index.json" {
					artifactRequests.Add(1)
					http.Error(writer, "artifact requested before identity validation", http.StatusBadRequest)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(writer).Encode(metadata); err != nil {
					t.Errorf("encode substituted release metadata: %v", err)
				}
			}))
			defer upstream.Close()

			directory := t.TempDir()
			want := version.Must(version.NewVersion("1.15.8"))
			_, err := installTerraform(context.Background(), want, directory, upstream.URL)
			if err == nil {
				t.Fatal("substituted release metadata was accepted")
			}
			if got := artifactRequests.Load(); got != 0 {
				t.Fatalf("requested %d artifacts before rejecting substituted release metadata", got)
			}
			if _, err := os.Stat(filepath.Join(directory, "terraform")); !os.IsNotExist(err) {
				t.Fatal("extracted Terraform before rejecting substituted release metadata")
			}
		})
	}
}

func terraformReleaseMetadata(version string) releaseMetadata {
	archive := "terraform_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
	checksums := "terraform_" + version + "_SHA256SUMS"
	return releaseMetadata{
		Name:       "terraform",
		Version:    version,
		Checksums:  checksums,
		Signature:  checksums + ".sig",
		Signatures: []string{checksums + ".72D7468F.sig", checksums + ".sig"},
		Builds: []releaseBuild{{
			Name:     "terraform",
			Version:  version,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Filename: archive,
			URL:      "https://releases.hashicorp.com/terraform/" + version + "/" + archive,
		}},
	}
}
