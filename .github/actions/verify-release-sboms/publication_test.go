package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftPublicationRejectsUnverifiedRemoteArtifacts(t *testing.T) {
	tests := []struct {
		name             string
		mutate           func(*testing.T, string, releaseFixture)
		draft            bool
		resignManifest   bool
		wrongFingerprint bool
		swapTarget       string
		swapAfter        string
		mutateInventory  func([]releaseAsset)
		wantErr          string
	}{
		{name: "valid signed draft", draft: true},
		{
			name: "remote asset has no immutable identity",
			mutateInventory: func(assets []releaseAsset) {
				assets[0].ID = ""
			},
			draft:   true,
			wantErr: "no immutable uploaded identity and SHA-256 digest",
		},
		{
			name: "remote asset has no immutable digest",
			mutateInventory: func(assets []releaseAsset) {
				assets[0].Digest = ""
			},
			draft:   true,
			wantErr: "no immutable uploaded identity and SHA-256 digest",
		},
		{
			name: "remote asset is not completely uploaded",
			mutateInventory: func(assets []releaseAsset) {
				assets[0].State = "starter"
			},
			draft:   true,
			wantErr: "no immutable uploaded identity and SHA-256 digest",
		},
		{
			name: "remote asset bytes do not match immutable digest",
			mutateInventory: func(assets []releaseAsset) {
				for index := range assets {
					if assets[index].Name == "terraform-provider-openai_1.2.3_SHA256SUMS" {
						assets[index].Digest = "sha256:" + strings.Repeat("0", sha256.Size*2)
					}
				}
			},
			draft:   true,
			wantErr: "does not match its immutable SHA-256 digest",
		},
		{
			name:       "checksum replaced after verified download",
			draft:      true,
			swapTarget: "terraform-provider-openai_1.2.3_SHA256SUMS",
			swapAfter:  "terraform-provider-openai_1.2.3_SHA256SUMS.sig",
			wantErr:    "uploaded release assets changed during verification",
		},
		{
			name:       "signature replaced after verified download",
			draft:      true,
			swapTarget: "terraform-provider-openai_1.2.3_SHA256SUMS.sig",
			wantErr:    "uploaded release assets changed during verification",
		},
		{
			name:       "archive replaced after verified download",
			draft:      true,
			swapTarget: "terraform-provider-openai_1.2.3_linux_amd64.zip",
			wantErr:    "uploaded release assets changed during verification",
		},
		{
			name:       "SPDX replaced after verified download",
			draft:      true,
			swapTarget: "terraform-provider-openai_1.2.3_linux_amd64.zip.spdx.json",
			wantErr:    "uploaded release assets changed during verification",
		},
		{
			name:       "registry replaced after verified download",
			draft:      true,
			swapTarget: "terraform-provider-openai_1.2.3_manifest.json",
			wantErr:    "uploaded release assets changed during verification",
		},
		{
			name: "signed release omits a configured platform and both checksum entries",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				archive := "terraform-provider-openai_1.2.3_linux_arm64.zip"
				for _, name := range []string{archive, archive + ".spdx.json"} {
					if err := os.Remove(filepath.Join(remote, name)); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(remote, filepath.Base(fixture.manifest))
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var retained []string
				for _, line := range strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n") {
					if !strings.HasSuffix(line, "  "+archive) && !strings.HasSuffix(line, "  "+archive+".spdx.json") {
						retained = append(retained, line)
					}
				}
				writeFixtureFile(t, path, []byte(strings.Join(retained, "\n")+"\n"))
			},
			draft:          true,
			resignManifest: true,
			wantErr:        "missing configured platform",
		},
		{
			name: "post-sign registry replacement refreshes signed checksums",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				replaceRemoteArtifact(t, remote, fixture, "terraform-provider-openai_1.2.3_manifest.json", []byte(`{"version":999,"metadata":{"protocol_versions":["6.0"]}}`))
			},
			draft:   true,
			wantErr: "verify uploaded checksum signature",
		},
		{
			name: "replaced provider archive",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				writeFixtureFile(t, filepath.Join(remote, filepath.Base(fixture.archive)), []byte("unverified uploaded provider"))
			},
			draft:   true,
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "malformed uploaded registry metadata",
			mutate: func(t *testing.T, remote string, _ releaseFixture) {
				writeFixtureFile(t, filepath.Join(remote, "terraform-provider-openai_1.2.3_manifest.json"), []byte(`{"version":999}`))
			},
			draft:   true,
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "signed malformed Terraform Registry metadata",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				replaceRemoteArtifact(t, remote, fixture, "terraform-provider-openai_1.2.3_manifest.json", []byte(`{"version":999,"metadata":{"protocol_versions":["6.0"]}}`))
			},
			draft:          true,
			resignManifest: true,
			wantErr:        "unsupported version 999",
		},
		{
			name: "signed malformed SPDX document",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				replaceRemoteArtifact(t, remote, fixture, filepath.Base(fixture.archive)+".spdx.json", []byte(`{"spdxVersion":"SPDX-2.3","packages":[]}`))
			},
			draft:          true,
			resignManifest: true,
			wantErr:        "has no version or packages",
		},
		{
			name: "signed unsupported archive platform",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				original := filepath.Base(fixture.archive)
				replacement := "terraform-provider-openai_1.2.3_linux_riscv64.zip"
				for _, suffix := range []string{"", ".spdx.json"} {
					if err := os.Rename(filepath.Join(remote, original+suffix), filepath.Join(remote, replacement+suffix)); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(remote, filepath.Base(fixture.manifest))
				manifest, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, path, []byte(strings.ReplaceAll(string(manifest), original, replacement)))
			},
			draft:          true,
			resignManifest: true,
			wantErr:        "unsupported release platform",
		},
		{
			name: "missing detached signature",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				if err := os.Remove(filepath.Join(remote, filepath.Base(fixture.manifest)+".sig")); err != nil {
					t.Fatal(err)
				}
			},
			draft:   true,
			wantErr: "missing signed checksum artifact",
		},
		{
			name: "forged detached signature",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				writeFixtureFile(t, filepath.Join(remote, filepath.Base(fixture.manifest)+".sig"), []byte("untrusted detached signature"))
			},
			draft:   true,
			wantErr: "verify uploaded checksum signature",
		},
		{
			name: "missing uploaded SPDX SBOM",
			mutate: func(t *testing.T, remote string, fixture releaseFixture) {
				if err := os.Remove(filepath.Join(remote, filepath.Base(fixture.archive)+".spdx.json")); err != nil {
					t.Fatal(err)
				}
			},
			draft:   true,
			wantErr: "has no matching SPDX SBOM",
		},
		{
			name: "missing uploaded Terraform Registry manifest",
			mutate: func(t *testing.T, remote string, _ releaseFixture) {
				if err := os.Remove(filepath.Join(remote, "terraform-provider-openai_1.2.3_manifest.json")); err != nil {
					t.Fatal(err)
				}
			},
			draft:   true,
			wantErr: "has no Terraform Registry manifest",
		},
		{
			name: "unexpected uploaded artifact",
			mutate: func(t *testing.T, remote string, _ releaseFixture) {
				writeFixtureFile(t, filepath.Join(remote, "unverified.json"), []byte("unverified"))
			},
			draft:   true,
			wantErr: "unexpected uploaded release artifact",
		},
		{name: "signature from an unexpected GPG key", draft: true, wrongFingerprint: true, wantErr: "verify uploaded checksum signature"},
		{name: "already public release", wantErr: "not a private draft"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReleaseFixture(t)
			remote := t.TempDir()
			paths, err := filepath.Glob(filepath.Join(fixture.directory, "*"))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range paths {
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, filepath.Join(remote, filepath.Base(path)), contents)
			}
			registry, err := os.ReadFile(fixture.registry)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, filepath.Join(remote, "terraform-provider-openai_1.2.3_manifest.json"), registry)
			writeFixtureFile(t, filepath.Join(remote, filepath.Base(fixture.manifest)+".sig"), []byte("trusted detached signature"))

			directory := t.TempDir()
			expectedManifest := filepath.Join(directory, "expected-manifest")
			manifest, err := os.ReadFile(fixture.manifest)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, expectedManifest, manifest)
			expectedSignature := filepath.Join(directory, "expected-signature")
			writeFixtureFile(t, expectedSignature, []byte("trusted detached signature"))
			if test.mutate != nil {
				test.mutate(t, remote, fixture)
			}
			if test.resignManifest {
				updated, err := os.ReadFile(filepath.Join(remote, filepath.Base(fixture.manifest)))
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, expectedManifest, updated)
			}

			entries, err := os.ReadDir(remote)
			if err != nil {
				t.Fatal(err)
			}
			assets := make([]releaseAsset, 0, len(entries))
			for _, entry := range entries {
				contents, err := os.ReadFile(filepath.Join(remote, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				assets = append(assets, releaseAsset{
					Name: entry.Name(), ID: "asset-" + entry.Name(),
					Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(contents)), State: "uploaded",
				})
			}
			if test.mutateInventory != nil {
				test.mutateInventory(assets)
			}
			description, err := json.Marshal(map[string]any{"tagName": "v1.2.3", "isDraft": test.draft, "assets": assets})
			if err != nil {
				t.Fatal(err)
			}
			descriptionPath := filepath.Join(directory, "release.json")
			writeFixtureFile(t, descriptionPath, description)
			if test.swapTarget != "" {
				replacement := []byte("unverified replacement after download")
				for index := range assets {
					if assets[index].Name == test.swapTarget {
						assets[index].ID = "replacement-" + test.swapTarget
						assets[index].Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(replacement))
					}
				}
				swapped, err := json.Marshal(map[string]any{"tagName": "v1.2.3", "isDraft": test.draft, "assets": assets})
				if err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, filepath.Join(directory, "swapped-release.json"), swapped)
				writeFixtureFile(t, filepath.Join(directory, "replacement"), replacement)
			}
			published := filepath.Join(directory, "published-release")

			gh := filepath.Join(directory, "gh")
			writeFixtureFile(t, gh, []byte("#!/bin/sh\n"+
				"if [ \"$1:$2\" = release:view ]; then cat \"$MOCK_RELEASE_DESCRIPTION\"; exit; fi\n"+
				"if [ \"$1:$2\" = release:edit ]; then printf '%s\\n' \"$@\" > \"$MOCK_PUBLISH_RECORD\"; exit; fi\n"+
				"if [ \"$1:$2\" = release:download ]; then\n"+
				"  shift 3\n"+
				"  while [ $# -gt 0 ]; do\n"+
				"    if [ \"$1\" = --pattern ]; then shift; asset=\"$1\"; fi\n"+
				"    shift\n"+
				"  done\n"+
				"  cat \"$MOCK_RELEASE_ASSETS/$asset\"\n"+
				"  if [ \"$asset\" = \"${MOCK_SWAP_AFTER:-}\" ]; then\n"+
				"    cp \"$MOCK_SWAP_SOURCE\" \"$MOCK_RELEASE_ASSETS/$MOCK_SWAP_TARGET\"\n"+
				"    cp \"$MOCK_SWAPPED_DESCRIPTION\" \"$MOCK_RELEASE_DESCRIPTION\"\n"+
				"  fi\n"+
				"  exit\n"+
				"fi\n"+
				"exit 2\n"))
			gpg := filepath.Join(directory, "gpg")
			writeFixtureFile(t, gpg, []byte("#!/bin/sh\n"+
				"test \"$3\" = \"$MOCK_SIGNER\" || exit 3\n"+
				"test \"$5\" = /dev/fd/3 || exit 6\n"+
				"cmp \"$5\" \"$MOCK_EXPECTED_SIGNATURE\" >/dev/null || exit 4\n"+
				"cmp - \"$MOCK_EXPECTED_MANIFEST\" >/dev/null || exit 5\n"))
			for _, path := range []string{gh, gpg} {
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("MOCK_RELEASE_DESCRIPTION", descriptionPath)
			t.Setenv("MOCK_RELEASE_ASSETS", remote)
			t.Setenv("MOCK_PUBLISH_RECORD", published)
			t.Setenv("MOCK_EXPECTED_MANIFEST", expectedManifest)
			t.Setenv("MOCK_EXPECTED_SIGNATURE", expectedSignature)
			t.Setenv("MOCK_SIGNER", "0123456789ABCDEF0123456789ABCDEF01234567")
			if test.swapTarget != "" {
				trigger := test.swapAfter
				if trigger == "" {
					trigger = test.swapTarget
				}
				t.Setenv("MOCK_SWAP_AFTER", trigger)
				t.Setenv("MOCK_SWAP_TARGET", test.swapTarget)
				t.Setenv("MOCK_SWAP_SOURCE", filepath.Join(directory, "replacement"))
				t.Setenv("MOCK_SWAPPED_DESCRIPTION", filepath.Join(directory, "swapped-release.json"))
			}

			fingerprint := "0123456789ABCDEF0123456789ABCDEF01234567"
			if test.wrongFingerprint {
				fingerprint = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
			}
			err = run([]string{"--publish", "v1.2.3", fingerprint})
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("verified private draft was not published: %v", err)
				}
				if _, err := os.Stat(published); err != nil {
					t.Fatalf("verified private draft did not reach publication: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("publication error = %v, want %q", err, test.wantErr)
			}
			if _, err := os.Stat(published); !os.IsNotExist(err) {
				t.Fatalf("unverified draft became publicly visible: %v", err)
			}
		})
	}
}

func replaceRemoteArtifact(t *testing.T, remote string, fixture releaseFixture, name string, contents []byte) {
	t.Helper()

	original, err := os.ReadFile(filepath.Join(remote, name))
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(remote, name), contents)
	manifestPath := filepath.Join(remote, filepath.Base(fixture.manifest))
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	before := fmt.Sprintf("%x", sha256.Sum256(original))
	after := fmt.Sprintf("%x", sha256.Sum256(contents))
	writeFixtureFile(t, manifestPath, []byte(strings.Replace(string(manifest), before+"  "+name, after+"  "+name, 1)))
}
