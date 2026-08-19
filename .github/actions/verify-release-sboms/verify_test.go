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
				for _, path := range []string{fixture.archive, fixture.archive + ".spdx.json"} {
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

	valid := newReleaseFixture(t)
	if err := run([]string{valid.manifest, "--sign", "--batch", "--detach-sign", valid.manifest}); err != nil {
		t.Fatalf("valid production artifacts could not be signed: %v", err)
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("GPG was not invoked for valid production artifacts: %v", err)
	}
	want := "--batch\n--detach-sign\n" + valid.manifest + "\n"
	if string(got) != want {
		t.Fatalf("GPG arguments = %q, want %q", got, want)
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
	publish := regexp.MustCompile(`(?ms)^  goreleaser:\n.*?^    environment: publish\n.*?^          args: release --clean\n`)
	if !publish.Match(workflow) {
		t.Fatal("approved production publishing job does not execute the verified GoReleaser release")
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

	build := strings.Index(publish, "      - name: Build trusted release verifier\n")
	secrets := strings.Index(publish, "      - name: Check publish environment secrets\n")
	importKey := strings.Index(publish, "      - name: Import GPG key\n")
	release := strings.Index(publish, "      - name: Run GoReleaser\n")
	if build < 0 || secrets < 0 || importKey < 0 || release < 0 || setupLocation[0] >= build || build >= secrets || secrets >= importKey || importKey >= release {
		t.Fatal("trusted release verifier is not built before signing secrets are exposed")
	}
	trustedBuild := publish[build:secrets]

	for _, command := range []string{
		`GOCACHE="$(mktemp -d "$RUNNER_TEMP/release-go-build-cache.XXXXXX")"`,
		`export GOCACHE`,
		`echo "GOCACHE=$GOCACHE" >> "$GITHUB_ENV"`,
		`go build -trimpath -o "$RUNNER_TEMP/release-verifier" .github/actions/verify-release-sboms/verify.go`,
		`echo "$RUNNER_TEMP" >> "$GITHUB_PATH"`,
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
}

func newReleaseFixture(t *testing.T) releaseFixture {
	t.Helper()

	directory := t.TempDir()
	archive := filepath.Join(directory, "terraform-provider-openai_1.2.3_linux_amd64.zip")
	fixture := releaseFixture{
		directory: directory,
		archive:   archive,
		manifest:  filepath.Join(directory, "terraform-provider-openai_1.2.3_SHA256SUMS"),
	}
	writeFixtureFile(t, archive, []byte("provider archive"))
	writeFixtureFile(t, archive+".spdx.json",
		[]byte(`{"spdxVersion":"SPDX-2.3","packages":[{"name":"terraform-provider-openai"}]}`))
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

	// GoReleaser signs the renamed Terraform Registry metadata even when that
	// metadata is not present as a file beside the generated release artifacts.
	fmt.Fprintf(&manifest, "%x  terraform-provider-openai_1.2.3_manifest.json\n",
		sha256.Sum256([]byte(`{"version":1}`)))
	writeFixtureFile(t, fixture.manifest, []byte(manifest.String()))
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
