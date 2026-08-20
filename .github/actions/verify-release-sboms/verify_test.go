package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVerifyRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*testing.T, releaseFixture)
		wantErr string
	}{
		{name: "valid archives, SPDX documents, and checksums"},
		{
			name: "multiple supported Terraform Registry protocol versions",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["5.0","6.0"]}}`))
				fixture.writeChecksums(t)
			},
		},
		{
			name: "supported freebsd arm release platform",
			mutate: func(t *testing.T, fixture releaseFixture) {
				if _, err := os.Stat(filepath.Join(fixture.directory, "terraform-provider-openai_1.2.3_freebsd_arm.zip")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "supported windows arm64 release platform",
			mutate: func(t *testing.T, fixture releaseFixture) {
				if _, err := os.Stat(filepath.Join(fixture.directory, "terraform-provider-openai_1.2.3_windows_arm64.zip")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "archive belongs to another provider",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-foreign_1.2.3_linux_amd64.zip")
			},
			wantErr: "does not belong to release",
		},
		{
			name: "archive belongs to another release version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_9.9.9_linux_amd64.zip")
			},
			wantErr: "does not belong to release",
		},
		{
			name: "archive has unsupported operating system",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_1.2.3_plan9_amd64.zip")
			},
			wantErr: "unsupported release platform",
		},
		{
			name: "archive has unsupported architecture",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_1.2.3_linux_riscv64.zip")
			},
			wantErr: "unsupported release platform",
		},
		{
			name: "archive uses ignored windows arm platform",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_1.2.3_windows_arm.zip")
			},
			wantErr: "unsupported release platform",
		},
		{
			name: "archive uses ignored darwin 386 platform",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_1.2.3_darwin_386.zip")
			},
			wantErr: "unsupported release platform",
		},
		{
			name: "archive has malformed release platform",
			mutate: func(t *testing.T, fixture releaseFixture) {
				renameFixtureArchive(t, fixture, "terraform-provider-openai_1.2.3_linux_amd64_extra.zip")
			},
			wantErr: "unsupported release platform",
		},
		{
			name: "mismatched archive digest",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.archive, []byte("tampered provider archive"))
			},
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "mismatched SBOM digest",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"tampered"}]}`))
			},
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "tampered Terraform Registry manifest source",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["5.0"]}}`))
			},
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "mismatched Terraform Registry manifest digest",
			mutate: func(t *testing.T, fixture releaseFixture) {
				name := "terraform-provider-openai_1.2.3_manifest.json"
				removeManifestEntry(t, fixture, name)
				appendManifestEntry(t, fixture, name, []byte("tampered registry manifest"))
			},
			wantErr: "SHA-256 checksum mismatch",
		},
		{
			name: "missing Terraform Registry manifest entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				removeManifestEntry(t, fixture, "terraform-provider-openai_1.2.3_manifest.json")
			},
			wantErr: "checksum manifest has no entry",
		},
		{
			name: "wrong Terraform Registry manifest release identity",
			mutate: func(t *testing.T, fixture releaseFixture) {
				removeManifestEntry(t, fixture, "terraform-provider-openai_1.2.3_manifest.json")
				appendManifestEntry(t, fixture, "terraform-provider-openai_9.9.9_manifest.json", []byte("different release"))
			},
			wantErr: "checksum manifest has no entry",
		},
		{
			name: "extra Terraform Registry manifest entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				appendManifestEntry(t, fixture, "terraform-provider-openai_9.9.9_manifest.json", []byte("different release"))
			},
			wantErr: "unexpected checksum manifest entry",
		},
		{
			name: "missing Terraform Registry manifest source",
			mutate: func(t *testing.T, fixture releaseFixture) {
				if err := os.Remove(fixture.registry); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "open release artifact",
		},
		{
			name: "malformed Terraform Registry manifest JSON",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":`))
				fixture.writeChecksums(t)
			},
			wantErr: "parse Terraform Registry manifest",
		},
		{
			name: "unsupported Terraform Registry manifest version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":2,"metadata":{"protocol_versions":["6.0"]}}`))
				fixture.writeChecksums(t)
			},
			wantErr: "unsupported version",
		},
		{
			name: "missing Terraform Registry protocol versions",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":[]}}`))
				fixture.writeChecksums(t)
			},
			wantErr: "has no protocol versions",
		},
		{
			name: "empty Terraform Registry protocol version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":[""]}}`))
				fixture.writeChecksums(t)
			},
			wantErr: "invalid protocol version",
		},
		{
			name: "malformed Terraform Registry protocol version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["6.x"]}}`))
				fixture.writeChecksums(t)
			},
			wantErr: "invalid protocol version",
		},
		{
			name: "duplicate Terraform Registry protocol version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["6.0","6.0"]}}`))
				fixture.writeChecksums(t)
			},
			wantErr: "duplicate protocol version",
		},
		{
			name: "unexpected checksum manifest entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				appendManifestEntry(t, fixture, "unexpected.json", []byte("unverified metadata"))
			},
			wantErr: "unexpected checksum manifest entry",
		},
		{
			name: "missing archive manifest entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				removeManifestEntry(t, fixture, filepath.Base(fixture.archive))
			},
			wantErr: "checksum manifest has no entry",
		},
		{
			name: "missing SBOM manifest entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				removeManifestEntry(t, fixture, filepath.Base(fixture.archive)+".spdx.json")
			},
			wantErr: "checksum manifest has no entry",
		},
		{
			name: "malformed SPDX JSON",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{"spdxVersion":`))
				fixture.writeChecksums(t)
			},
			wantErr: "parse SPDX SBOM",
		},
		{
			name: "missing SPDX version",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{"packages":[{"name":"provider"}]}`))
				fixture.writeChecksums(t)
			},
			wantErr: "has no version or packages",
		},
		{
			name: "empty SPDX packages",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{"spdxVersion":"SPDX-2.3","packages":[]}`))
				fixture.writeChecksums(t)
			},
			wantErr: "has no version or packages",
		},
		{
			name: "missing SBOM",
			mutate: func(t *testing.T, fixture releaseFixture) {
				if err := os.Remove(fixture.archive + ".spdx.json"); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "archives but",
		},
		{
			name: "extra SBOM",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, filepath.Join(fixture.directory, "unexpected.zip.spdx.json"),
					[]byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"unexpected"}]}`))
				fixture.writeChecksums(t)
			},
			wantErr: "archives but",
		},
		{
			name: "unpaired SBOM",
			mutate: func(t *testing.T, fixture releaseFixture) {
				if err := os.Rename(fixture.archive+".spdx.json", filepath.Join(fixture.directory, "unexpected.zip.spdx.json")); err != nil {
					t.Fatal(err)
				}
				fixture.writeChecksums(t)
			},
			wantErr: "no matching adjacent SBOM",
		},
		{
			name: "manifest lists missing archive",
			mutate: func(t *testing.T, fixture releaseFixture) {
				appendManifestEntry(t, fixture, "missing.zip", []byte("missing archive"))
			},
			wantErr: "checksum manifest lists missing archive",
		},
		{
			name: "manifest lists missing SBOM",
			mutate: func(t *testing.T, fixture releaseFixture) {
				appendManifestEntry(t, fixture, "missing.zip.spdx.json", []byte("missing SBOM"))
			},
			wantErr: "checksum manifest lists missing SBOM",
		},
		{
			name: "duplicate checksum entry",
			mutate: func(t *testing.T, fixture releaseFixture) {
				contents, err := os.ReadFile(fixture.archive)
				if err != nil {
					t.Fatal(err)
				}
				appendManifestEntry(t, fixture, filepath.Base(fixture.archive), contents)
			},
			wantErr: "duplicate checksum manifest entry",
		},
		{
			name: "malformed checksum digest",
			mutate: func(t *testing.T, fixture releaseFixture) {
				writeFixtureFile(t, fixture.manifest, []byte("not-a-sha256-digest  provider.zip\n"))
			},
			wantErr: "malformed SHA-256 checksum manifest entry",
		},
		{
			name: "manifest path traversal",
			mutate: func(t *testing.T, fixture releaseFixture) {
				appendManifestEntry(t, fixture, "../provider.zip", []byte("outside archive"))
			},
			wantErr: "not an artifact filename",
		},
		{
			name: "no archives",
			mutate: func(t *testing.T, fixture releaseFixture) {
				paths, err := filepath.Glob(filepath.Join(fixture.directory, "*.zip*"))
				if err != nil {
					t.Fatal(err)
				}
				for _, path := range paths {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			},
			wantErr: "release contains no provider archives",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newReleaseFixture(t)
			if test.mutate != nil {
				test.mutate(t, fixture)
			}
			err := verifyRelease(fixture.manifest)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("valid release was rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("verifyRelease() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestVerifyReleaseRejectsChecksumWithoutReleaseIdentity(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t)
	unexpected := filepath.Join(fixture.directory, "checksums.txt")
	if err := os.Rename(fixture.manifest, unexpected); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelease(unexpected); err == nil || !strings.Contains(err.Error(), "has no release identity") {
		t.Fatalf("checksum manifest without a release identity was accepted: %v", err)
	}
}

func TestVerifyReleaseRejectsForeignChecksumProject(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t)
	foreign := filepath.Join(fixture.directory, "terraform-provider-foreign_1.2.3_SHA256SUMS")
	if err := os.Rename(fixture.manifest, foreign); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelease(foreign); err == nil || !strings.Contains(err.Error(), "unsupported provider release identity") {
		t.Fatalf("checksum manifest for another Terraform provider was accepted: %v", err)
	}
}

func TestSigningRequiresVerifiedProductionArtifacts(t *testing.T) {
	directory := t.TempDir()
	record := filepath.Join(directory, "gpg-invocation")
	writeFixtureFile(t, filepath.Join(directory, "gpg"),
		[]byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SIGNING_RECORD\"\n"))
	if err := os.Chmod(filepath.Join(directory, "gpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SIGNING_RECORD", record)

	verified := newReleaseFixture(t)
	unverified := newReleaseFixture(t)
	if err := run([]string{verified.manifest, "--sign", "--batch", "--detach-sign", unverified.manifest}); err == nil ||
		!strings.Contains(err.Error(), "signing target does not match verified checksum manifest") {
		t.Fatalf("signing target was not bound to the verified manifest: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("GPG was invoked for a different unverified checksum manifest: %v", err)
	}

	invalid := newReleaseFixture(t)
	writeFixtureFile(t, invalid.archive, []byte("tampered production archive"))
	if err := run([]string{invalid.manifest, "--sign", "--batch", "--detach-sign", invalid.manifest}); err == nil || !strings.Contains(err.Error(), "SHA-256 checksum mismatch") {
		t.Fatalf("tampered production artifacts reached signing: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("GPG was invoked for unverified production artifacts: %v", err)
	}

	invalidRegistry := newReleaseFixture(t)
	writeFixtureFile(t, invalidRegistry.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["5.0"]}}`))
	if err := run([]string{invalidRegistry.manifest, "--sign", "--batch", "--detach-sign", invalidRegistry.manifest}); err == nil ||
		!strings.Contains(err.Error(), "SHA-256 checksum mismatch") {
		t.Fatalf("tampered Terraform Registry metadata reached signing: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("GPG was invoked for unverified Terraform Registry metadata: %v", err)
	}

	valid := newReleaseFixture(t)
	if err := run([]string{valid.manifest, "--sign", "--batch", "--detach-sign", valid.manifest}); err != nil {
		t.Fatalf("valid production artifacts could not be signed: %v", err)
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("GPG was not invoked for valid production artifacts: %v", err)
	}
	want := "--batch\n--detach-sign\n-\n"
	if string(got) != want {
		t.Fatalf("GPG arguments = %q, want %q", got, want)
	}
}

func TestSigningBindsVerifiedChecksumSnapshot(t *testing.T) {
	directory := t.TempDir()
	fixture := newReleaseFixture(t)
	verified, err := os.ReadFile(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile(fixture.archive)
	if err != nil {
		t.Fatal(err)
	}
	forged := strings.Replace(string(verified), fmt.Sprintf("%x", sha256.Sum256(archive)), strings.Repeat("0", 64), 1)
	forgedPath := filepath.Join(directory, "forged-checksums")
	signedPath := filepath.Join(directory, "signed-checksums")
	writeFixtureFile(t, forgedPath, []byte(forged))

	gpg := filepath.Join(directory, "gpg")
	writeFixtureFile(t, gpg, []byte("#!/bin/sh\n"+
		"mv \"$FORGED_CHECKSUMS\" \"$PUBLISHED_CHECKSUMS\"\n"+
		"for argument in \"$@\"; do target=\"$argument\"; done\n"+
		"if [ \"$target\" = - ]; then cat > \"$SIGNED_CHECKSUMS\"; else cat \"$target\" > \"$SIGNED_CHECKSUMS\"; fi\n"))
	if err := os.Chmod(gpg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FORGED_CHECKSUMS", forgedPath)
	t.Setenv("PUBLISHED_CHECKSUMS", fixture.manifest)
	t.Setenv("SIGNED_CHECKSUMS", signedPath)

	if err := run([]string{fixture.manifest, "--sign", "--batch", "--detach-sign", fixture.manifest}); err != nil {
		t.Fatalf("could not sign verified checksum manifest: %v", err)
	}
	signed, err := os.ReadFile(signedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(signed) != string(verified) {
		t.Fatalf("GPG signed bytes other than the verified checksum snapshot: %q", signed)
	}
	published, err := os.ReadFile(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(published) != string(verified) {
		t.Fatalf("published checksum manifest differs from its verified signed snapshot: %q", published)
	}
	publishedInfo, err := os.Stat(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	if publishedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("published checksum permissions = %o, want %o", publishedInfo.Mode().Perm(), originalInfo.Mode().Perm())
	}
	snapshots, err := filepath.Glob(filepath.Join(fixture.directory, ".verified-*"))
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("temporary verified checksum snapshots were not cleaned up: %v, %v", snapshots, err)
	}
}

func TestSigningRestoresValidatedArtifactSnapshots(t *testing.T) {
	tests := []struct {
		name      string
		artifact  func(releaseFixture) string
		malformed string
	}{
		{
			name:      "provider ZIP archive",
			artifact:  func(fixture releaseFixture) string { return fixture.archive },
			malformed: "replaced provider binary archive",
		},
		{
			name:      "Terraform Registry manifest",
			artifact:  func(fixture releaseFixture) string { return fixture.registry },
			malformed: `{"version":1,"metadata":{"protocol_versions":["not-a-protocol"]}}`,
		},
		{
			name:      "SPDX SBOM",
			artifact:  func(fixture releaseFixture) string { return fixture.archive + ".spdx.json" },
			malformed: `{"spdxVersion":"SPDX-2.3","packages":[]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			fixture := newReleaseFixture(t)
			path := test.artifact(fixture)
			verified, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			originalInfo, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			malformed := filepath.Join(directory, "malformed-artifact")
			writeFixtureFile(t, malformed, []byte(test.malformed))
			record := filepath.Join(directory, "gpg-invocation")
			gpg := filepath.Join(directory, "gpg")
			writeFixtureFile(t, gpg, []byte("#!/bin/sh\n"+
				"mv \"$MALFORMED_ARTIFACT\" \"$PUBLISHED_ARTIFACT\"\n"+
				"cat > /dev/null\nprintf 'signed\\n' > \"$SIGNING_RECORD\"\n"))
			if err := os.Chmod(gpg, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("MALFORMED_ARTIFACT", malformed)
			t.Setenv("PUBLISHED_ARTIFACT", path)
			t.Setenv("SIGNING_RECORD", record)

			if err := run([]string{fixture.manifest, "--sign", "--batch", "--detach-sign", fixture.manifest}); err != nil {
				t.Fatalf("could not sign verified release artifacts: %v", err)
			}
			if _, err := os.Stat(record); err != nil {
				t.Fatalf("verified release artifacts did not reach GPG: %v", err)
			}
			published, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(published) != string(verified) {
				t.Fatalf("published artifact differs from its verified digest snapshot: %q", published)
			}
			publishedInfo, err := os.Stat(path)
			if err != nil || publishedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
				t.Fatalf("published artifact permissions differ from their verified snapshot: %v", err)
			}
		})
	}
}

func TestMetadataValidationUsesItsHashedSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		artifact  func(releaseFixture) (string, string)
		malformed string
		valid     string
		validate  func(string, []byte) error
		wantErr   string
	}{
		{
			name: "Terraform Registry manifest",
			artifact: func(fixture releaseFixture) (string, string) {
				return "terraform-provider-openai_1.2.3_manifest.json", fixture.registry
			},
			malformed: `{"version":1,"metadata":{"protocol_versions":["not-a-protocol"]}}`,
			valid:     `{"version":1,"metadata":{"protocol_versions":["6.0"]}}`,
			validate:  verifyRegistryManifest,
			wantErr:   "invalid protocol version",
		},
		{
			name: "SPDX SBOM",
			artifact: func(fixture releaseFixture) (string, string) {
				return filepath.Base(fixture.archive) + ".spdx.json", fixture.archive + ".spdx.json"
			},
			malformed: `{"spdxVersion":"SPDX-2.3","packages":[]}`,
			valid:     `{"spdxVersion":"SPDX-2.3","packages":[{"name":"provider"}]}`,
			validate: func(path string, document []byte) error {
				_, err := parseSPDX(path, document)
				return err
			},
			wantErr: "has no version or packages",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newReleaseFixture(t)
			name, path := test.artifact(fixture)
			writeFixtureFile(t, path, []byte(test.malformed))
			fixture.writeChecksums(t)
			manifest, err := os.ReadFile(fixture.manifest)
			if err != nil {
				t.Fatal(err)
			}
			checksums, err := readChecksums(manifest)
			if err != nil {
				t.Fatal(err)
			}
			replacement := filepath.Join(t.TempDir(), "structurally-valid-replacement")
			writeFixtureFile(t, replacement, []byte(test.valid))

			_, err = verifyMetadataArtifact(name, path, checksums, func(filename string, snapshot []byte) error {
				if renameErr := os.Rename(replacement, path); renameErr != nil {
					return renameErr
				}
				return test.validate(filename, snapshot)
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("structurally valid pathname replacement bypassed malformed hashed bytes: %v", err)
			}
		})
	}
}

func TestSigningRejectsUnsupportedReleaseArchives(t *testing.T) {
	tests := []struct {
		name    string
		archive string
	}{
		{name: "another provider", archive: "terraform-provider-foreign_1.2.3_linux_amd64.zip"},
		{name: "another release version", archive: "terraform-provider-openai_9.9.9_linux_amd64.zip"},
		{name: "unsupported windows arm", archive: "terraform-provider-openai_1.2.3_windows_arm.zip"},
		{name: "unsupported darwin 386", archive: "terraform-provider-openai_1.2.3_darwin_386.zip"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			record := filepath.Join(directory, "gpg-invocation")
			gpg := filepath.Join(directory, "gpg")
			writeFixtureFile(t, gpg, []byte("#!/bin/sh\nprintf 'signed\\n' > \"$SIGNING_RECORD\"\n"))
			if err := os.Chmod(gpg, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SIGNING_RECORD", record)

			fixture := newReleaseFixture(t)
			renameFixtureArchive(t, fixture, test.archive)
			if err := run([]string{fixture.manifest, "--sign", "--batch", "--detach-sign", fixture.manifest}); err == nil {
				t.Fatal("unsupported archive reached production signing")
			}
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatalf("GPG signed an unsupported production archive: %v", err)
			}
		})
	}
}

func TestProductionReleaseUsesVerifiedSigner(t *testing.T) {
	t.Parallel()

	repository := filepath.Join("..", "..", "..")
	configuration, err := os.ReadFile(filepath.Join(repository, ".goreleaser.yml"))
	if err != nil {
		t.Fatal(err)
	}
	signer := regexp.MustCompile(`(?m)^signs:\n  - artifacts: checksum\n    cmd: release-verifier\n    args:\n      - "\$\{artifact\}"\n      - --sign\n`)
	if !signer.Match(configuration) {
		t.Fatal("GoReleaser production checksum signing does not use the trusted prebuilt verifier")
	}

	workflow, err := os.ReadFile(filepath.Join(repository, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	publish := regexp.MustCompile(`(?ms)^  goreleaser:\n.*?^    environment: publish\n.*?^      - name: Run GoReleaser\n.*?^          "\$RELEASE_TOOLS_DIR/goreleaser" release --clean --draft\n.*?^      - name: Verify and publish release draft\n`)
	if !publish.Match(workflow) {
		t.Fatal("approved production publishing job does not verify the private GoReleaser draft before publication")
	}
	if !strings.Contains(string(workflow), `release-verifier --publish "$GITHUB_REF_NAME" "$GPG_FINGERPRINT"`) {
		t.Fatal("production publishing job does not require the trusted release-wide publication verifier")
	}
}

func TestProductionSigningDoesNotRestoreSharedGoCache(t *testing.T) {
	t.Parallel()

	workflow, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	publishStart := strings.Index(string(workflow), "\n  goreleaser:\n")
	if publishStart < 0 {
		t.Fatal("production publishing job was not found")
	}
	publish := string(workflow[publishStart:])

	setup := regexp.MustCompile(`(?m)^      - uses: actions/setup-go@[^\n]+\n        with:\n          go-version-file: go\.mod\n          cache: false\n`)
	setupLocation := setup.FindStringIndex(publish)
	if setupLocation == nil {
		t.Fatal("production publishing job restores a shared Go build cache")
	}

	tools := strings.Index(publish, "      - name: Install authenticated release tools\n")
	dependencies := strings.Index(publish, "      - name: Verify release dependencies\n")
	build := strings.Index(publish, "      - name: Build trusted release verifier\n")
	secrets := strings.Index(publish, "      - name: Check publish environment secrets\n")
	importKey := strings.Index(publish, "      - name: Import GPG key\n")
	release := strings.Index(publish, "      - name: Run GoReleaser\n")
	if tools < 0 || dependencies < 0 || build < 0 || secrets < 0 || importKey < 0 || release < 0 ||
		setupLocation[0] >= tools || tools >= dependencies || dependencies >= build ||
		build >= secrets || secrets >= importKey || importKey >= release {
		t.Fatal("trusted release tools, dependencies, and verifier are not authenticated before signing secrets are exposed")
	}
	trustedBuild := publish[build:secrets]

	for _, command := range []string{
		`GOCACHE="$(mktemp -d "$RUNNER_TEMP/release-go-build-cache.XXXXXX")"`,
		`export GOCACHE`,
		`echo "GOCACHE=$GOCACHE" >> "$GITHUB_ENV"`,
		`go build -trimpath -o "$RUNNER_TEMP/release-verifier" .github/actions/verify-release-sboms/verify.go`,
		`echo "$RUNNER_TEMP" >> "$GITHUB_PATH"`,
		`GONOPROXY: "none"`,
		`GOPROXY: "off"`,
	} {
		if !strings.Contains(trustedBuild, command) {
			t.Errorf("production publishing job does not enforce isolated signing prerequisite %q", command)
		}
	}
}

func TestPrebuiltSignerIgnoresHostileGoBuildCache(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	verifier := filepath.Join(directory, "release-verifier")
	build := exec.Command("go", "build", "-trimpath", "-o", verifier, "verify.go")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build trusted release verifier before signing: %v\n%s", err, output)
	}

	signingRecord := filepath.Join(directory, "gpg-invocation")
	goRecord := filepath.Join(directory, "hostile-go-invocation")
	for name, source := range map[string]string{
		"gpg": "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SIGNING_RECORD\"\n",
		"go":  "#!/bin/sh\nprintf 'hostile Go toolchain executed\\n' > \"$GO_INVOCATION_RECORD\"\nexit 90\n",
	} {
		path := filepath.Join(directory, name)
		writeFixtureFile(t, path, []byte(source))
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	fixture := newReleaseFixture(t)
	sign := exec.Command(verifier, fixture.manifest, "--sign", "--batch", "--detach-sign", fixture.manifest)
	sign.Env = append(os.Environ(),
		"PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOCACHE="+filepath.Join(directory, "hostile-shared-cache"),
		"SIGNING_RECORD="+signingRecord,
		"GO_INVOCATION_RECORD="+goRecord,
	)
	if output, err := sign.CombinedOutput(); err != nil {
		t.Fatalf("trusted verifier rejected valid production artifacts: %v\n%s", err, output)
	}
	if _, err := os.Stat(goRecord); !os.IsNotExist(err) {
		t.Fatalf("production signer executed the hostile Go toolchain: %v", err)
	}
	if _, err := os.Stat(signingRecord); err != nil {
		t.Fatalf("trusted verifier did not sign valid production artifacts: %v", err)
	}
}

type releaseFixture struct {
	directory string
	archive   string
	manifest  string
	registry  string
}

func newReleaseFixture(t *testing.T) releaseFixture {
	t.Helper()

	root := t.TempDir()
	directory := filepath.Join(root, "dist")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(directory, "terraform-provider-openai_1.2.3_linux_amd64.zip")
	fixture := releaseFixture{
		directory: directory,
		archive:   archive,
		manifest:  filepath.Join(directory, "terraform-provider-openai_1.2.3_SHA256SUMS"),
		registry:  filepath.Join(root, "terraform-registry-manifest.json"),
	}
	writeFixtureFile(t, fixture.registry, []byte(`{"version":1,"metadata":{"protocol_versions":["6.0"]}}`))
	for _, platform := range releasePlatforms {
		path := filepath.Join(directory, "terraform-provider-openai_1.2.3_"+platform+".zip")
		writeReleaseFixtureArchive(t, path)
	}
	fixture.writeChecksums(t)
	return fixture
}

func (fixture releaseFixture) writeChecksums(t *testing.T) {
	t.Helper()

	entries, err := os.ReadDir(fixture.directory)
	if err != nil {
		t.Fatal(err)
	}
	var manifest strings.Builder
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".zip") && !strings.HasSuffix(entry.Name(), ".spdx.json") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(fixture.directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(contents), entry.Name())
	}

	registry, err := os.ReadFile(fixture.registry)
	if err != nil {
		t.Fatal(err)
	}
	// GoReleaser signs the renamed Terraform Registry metadata while its source
	// remains in the repository root instead of beside the release artifacts.
	fmt.Fprintf(&manifest, "%x  terraform-provider-openai_1.2.3_manifest.json\n",
		sha256.Sum256(registry))
	writeFixtureFile(t, fixture.manifest, []byte(manifest.String()))
}

func renameFixtureArchive(t *testing.T, fixture releaseFixture, name string) {
	t.Helper()

	replacement := filepath.Join(fixture.directory, name)
	if err := os.Rename(fixture.archive, replacement); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(fixture.archive+".spdx.json", replacement+".spdx.json"); err != nil {
		t.Fatal(err)
	}
	fixture.writeChecksums(t)
}

func removeManifestEntry(t *testing.T, fixture releaseFixture, name string) {
	t.Helper()

	contents, err := os.ReadFile(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	var retained []string
	for _, line := range strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n") {
		if !strings.HasSuffix(line, "  "+name) {
			retained = append(retained, line)
		}
	}
	writeFixtureFile(t, fixture.manifest, []byte(strings.Join(retained, "\n")+"\n"))
}

func appendManifestEntry(t *testing.T, fixture releaseFixture, name string, contents []byte) {
	t.Helper()

	manifest, err := os.OpenFile(fixture.manifest, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(manifest, "%x  %s\n", sha256.Sum256(contents), name); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}
