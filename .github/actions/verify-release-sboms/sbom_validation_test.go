package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigningRejectsReplacedInternallyChecksummedSBOM(t *testing.T) {
	fixture := newReleaseFixture(t)
	writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{"spdxVersion":"SPDX-2.3","name":"attacker-rebound-provider","packages":[{"name":"attacker-controlled"}]}`))
	fixture.writeChecksums(t)

	if err := verifyRelease(fixture.manifest); err == nil || !strings.Contains(err.Error(), "SPDX") {
		t.Fatalf("internally checksummed replacement SBOM was not rejected: %v", err)
	}
}

func TestSigningRejectsArchiveReplacedAfterSBOMGeneration(t *testing.T) {
	fixture := newReleaseFixture(t)
	archive, err := os.ReadFile(fixture.archive)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, fixture.archive, append(archive, []byte("post-generation replacement")...))
	fixture.writeChecksums(t)

	err = verifyRelease(fixture.manifest)
	if err == nil || !strings.Contains(err.Error(), "invalid archive identity, SHA-256 digest, or supplier") {
		t.Fatalf("internally checksummed archive replacement was not rejected: %v", err)
	}
}

func TestSigningRevalidatesCompleteArchiveSBOM(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr string
	}{
		{
			name: "document archive identity",
			mutate: func(document map[string]any) {
				document["name"] = "attacker-rebound-provider.zip"
			},
			wantErr: "does not identify archive",
		},
		{
			name: "missing described archive",
			mutate: func(document map[string]any) {
				document["relationships"] = []any{}
			},
			wantErr: "does not describe its provider archive",
		},
		{
			name: "multiple described archives",
			mutate: func(document map[string]any) {
				relationships := document["relationships"].([]any)
				document["relationships"] = append(relationships, relationships[0])
			},
			wantErr: "describes more than one provider archive",
		},
		{
			name: "provider archive identity",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[0].(map[string]any)["name"] = "attacker-rebound-provider.zip"
			},
			wantErr: "invalid archive identity",
		},
		{
			name: "provider archive digest",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[0].(map[string]any)["versionInfo"] = "sha256:attacker-controlled"
			},
			wantErr: "invalid archive identity",
		},
		{
			name: "provider source supplier",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[0].(map[string]any)["supplier"] = "Attacker Controlled"
			},
			wantErr: "invalid archive identity",
		},
		{
			name: "missing provider archive package",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[0].(map[string]any)["SPDXID"] = "SPDXRef-attacker"
			},
			wantErr: "no matching provider archive package",
		},
		{
			name: "missing provider dependency",
			mutate: func(document map[string]any) {
				document["packages"] = document["packages"].([]any)[:2]
			},
			wantErr: "omits provider dependency",
		},
		{
			name: "forged provider dependency version",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[2].(map[string]any)["versionInfo"] = "v999.999.999"
			},
			wantErr: "omits provider dependency",
		},
		{
			name: "forged OpenAI SDK license",
			mutate: func(document map[string]any) {
				document["packages"].([]any)[1].(map[string]any)["licenseConcluded"] = "MIT"
			},
			wantErr: "no Apache-2.0 OpenAI SDK package",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReleaseFixture(t)
			mutateFixtureSBOM(t, fixture, test.mutate)

			directory := t.TempDir()
			record := filepath.Join(directory, "signing-record")
			gpg := filepath.Join(directory, "gpg")
			writeFixtureFile(t, gpg, []byte("#!/bin/sh\nprintf 'signed\\n' > \"$SIGNING_RECORD\"\n"))
			if err := os.Chmod(gpg, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SIGNING_RECORD", record)

			err := run([]string{fixture.manifest, "--sign", "--batch", "--detach-sign", fixture.manifest})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("signing error = %v, want %q", err, test.wantErr)
			}
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatalf("unverified SBOM reached production GPG signing: %v", err)
			}
		})
	}
}

func mutateFixtureSBOM(t *testing.T, fixture releaseFixture, mutate func(map[string]any)) {
	t.Helper()

	contents, err := os.ReadFile(fixture.archive + ".spdx.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	mutate(document)
	updated, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, fixture.archive+".spdx.json", updated)
	fixture.writeChecksums(t)
}

func TestGPGVerificationFailureRedactsSignerIdentity(t *testing.T) {
	directory := t.TempDir()
	gpg := filepath.Join(directory, "gpg")
	writeFixtureFile(t, gpg, []byte("#!/bin/sh\n"+
		"printf 'gpg: Good signature from \"Synthetic Release Signer <signer@example.invalid>\"\\n' >&2\n"+
		"printf 'gpg: using EDDSA key ABCDEF0123456789ABCDEF0123456789ABCDEF01\\n' >&2\n"+
		"exit 1\n"))
	if err := os.Chmod(gpg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := verifyReleaseSignature([]byte("signed checksum manifest"), []byte("signature"), "UNEXPECTED-FINGERPRINT")
	if err == nil {
		t.Fatal("unexpected signer identity was accepted")
	}
	for _, sensitive := range []string{"signer@example.invalid", "Synthetic Release Signer", "ABCDEF0123456789"} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("GPG verification failure exposes signer identity %q: %v", sensitive, err)
		}
	}
}
