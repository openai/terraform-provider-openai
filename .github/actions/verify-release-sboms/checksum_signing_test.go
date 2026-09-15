package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigningProducesSeparateVerifiableInventories(t *testing.T) {
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		t.Fatal("gpg is required to verify the release signing contract")
	}
	// Use only a disposable test key; never read the developer's keyring.
	// Keep the directory short enough for gpg-agent's Unix socket on macOS.
	home, err := os.MkdirTemp("", "tf-gpg-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("GNUPGHOME", home)
	key := "Release fixture <release@example.invalid>"
	generate := exec.Command(gpg, "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-generate-key", key, "rsa2048", "sign", "0")
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate disposable release key: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("gpgconf", "--homedir", home, "--kill", "gpg-agent").Run()
	})
	fixture := newReleaseFixture(t)
	original, err := os.ReadFile(fixture.manifest)
	if err != nil {
		t.Fatal(err)
	}
	registry, _, err := splitReleaseChecksums(original)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, fixture.manifest, registry)
	if err := run([]string{fixture.manifest, "--sign", "--batch", "--local-user", key,
		"--output", fixture.manifest + ".sig", "--detach-sign", fixture.manifest}); err != nil {
		t.Fatalf("sign complete release inventory: %v", err)
	}
	// GoReleaser refreshes its checksum artifact after signing. Its configured
	// Registry inventory must remain byte-identical to what we signed.
	writeFixtureFile(t, fixture.manifest, registry)
	sbomPath := strings.TrimSuffix(fixture.manifest, "_SHA256SUMS") + "_sbom_checksums.txt"
	union := make(map[string][]byte)
	for _, path := range []string{fixture.manifest, sbomPath} {
		verify := exec.Command(gpg, "--batch", "--verify", path+".sig", path)
		if output, err := verify.CombinedOutput(); err != nil {
			t.Fatalf("verify %s signature: %v\n%s", filepath.Base(path), err, output)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := readChecksums(contents)
		if err != nil {
			t.Fatal(err)
		}
		wantCount := 13
		if path == fixture.manifest {
			wantCount = 14
		}
		if len(entries) != wantCount {
			t.Errorf("%s entries = %d, want %d", filepath.Base(path), len(entries), wantCount)
		}
		for name, digest := range entries {
			if path == fixture.manifest && !strings.HasSuffix(name, ".zip") && name != "terraform-provider-openai_1.2.3_manifest.json" {
				t.Errorf("Registry inventory contains unsupported artifact %q", name)
			}
			if path == sbomPath && !strings.HasSuffix(name, ".zip.spdx.json") {
				t.Errorf("SBOM inventory contains unsupported artifact %q", name)
			}
			if _, exists := union[name]; exists {
				t.Errorf("artifact %q is in both checksum inventories", name)
			}
			union[name] = digest
		}
		// A signature for one inventory must not validate the other inventory.
		other := fixture.manifest
		if path == other {
			other = sbomPath
		}
		if err := exec.Command(gpg, "--batch", "--verify", path+".sig", other).Run(); err == nil {
			t.Errorf("signature for %s accepted different inventory %s", path, other)
		}
	}
	want, err := readChecksums(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(union) != len(want) {
		t.Fatalf("signed inventory union has %d artifacts, want %d", len(union), len(want))
	}
	for name, digest := range want {
		if !bytes.Equal(union[name], digest) {
			t.Errorf("signed checksum for %q differs from the verified original", name)
		}
	}
}

func TestRegistryInventoryRequiresValidAdjacentSBOMs(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, releaseFixture)
	}{
		{"missing", func(t *testing.T, fixture releaseFixture) {
			if err := os.Remove(fixture.archive + ".spdx.json"); err != nil {
				t.Fatal(err)
			}
		}},
		{"invalid", func(t *testing.T, fixture releaseFixture) {
			writeFixtureFile(t, fixture.archive+".spdx.json", []byte(`{}`))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReleaseFixture(t)
			full, err := os.ReadFile(fixture.manifest)
			if err != nil {
				t.Fatal(err)
			}
			registry, _, err := splitReleaseChecksums(full)
			if err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, fixture.manifest, registry)
			test.mutate(t, fixture)
			if err := verifyRelease(fixture.manifest); err == nil {
				t.Fatal("Registry-only checksums bypassed SBOM verification")
			}
		})
	}
}
