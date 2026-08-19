package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || len(args) > 1 && (args[1] != "--sign" || len(args) == 2) {
		return errors.New("usage: verify.go CHECKSUM_FILE [--sign GPG_ARGS...]")
	}
	if len(args) > 1 && args[len(args)-1] != args[0] {
		return errors.New("signing target does not match verified checksum manifest")
	}

	manifest, mode, err := readArtifactSnapshot(args[0])
	if err != nil {
		return err
	}
	var metadata []verifiedArtifact
	if err := verifyReleaseSnapshot(args[0], manifest, &metadata); err != nil {
		return err
	}
	if len(args) == 1 {
		return nil
	}

	signingArguments := append([]string(nil), args[2:]...)
	signingArguments[len(signingArguments)-1] = "-"
	sign := exec.Command("gpg", signingArguments...)
	sign.Stdin = bytes.NewReader(manifest)
	sign.Stdout = os.Stdout
	sign.Stderr = os.Stderr
	if err := sign.Run(); err != nil {
		return fmt.Errorf("sign verified checksum manifest: %w", err)
	}
	for _, artifact := range metadata {
		if err := publishVerifiedSnapshot(artifact.path, artifact.contents, artifact.mode); err != nil {
			return fmt.Errorf("publish verified release metadata %q: %w", artifact.name, err)
		}
	}
	return publishVerifiedSnapshot(args[0], manifest, mode)
}

func verifyRelease(checksumPath string) error {
	return run([]string{checksumPath})
}

func verifyReleaseSnapshot(checksumPath string, manifest []byte, metadata *[]verifiedArtifact) error {
	checksums, err := readChecksums(manifest)
	if err != nil {
		return err
	}

	archives := make(map[string]string)
	sboms := make(map[string]string)
	if err := filepath.WalkDir(filepath.Dir(checksumPath), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		var artifacts map[string]string
		switch {
		case strings.HasSuffix(entry.Name(), ".zip"):
			artifacts = archives
		case strings.HasSuffix(entry.Name(), ".spdx.json"):
			artifacts = sboms
		default:
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("release artifact %q is not a regular file", path)
		}
		if _, exists := artifacts[entry.Name()]; exists {
			return fmt.Errorf("duplicate release artifact name %q", entry.Name())
		}
		artifacts[entry.Name()] = path
		return nil
	}); err != nil {
		return fmt.Errorf("discover release artifacts: %w", err)
	}

	if len(archives) == 0 {
		return errors.New("release contains no provider archives")
	}
	if len(sboms) != len(archives) {
		return fmt.Errorf("release has %d archives but %d SBOMs", len(archives), len(sboms))
	}

	releaseName, valid := strings.CutSuffix(filepath.Base(checksumPath), "_SHA256SUMS")
	if !valid || releaseName == "" {
		return fmt.Errorf("checksum manifest %q has no release identity", filepath.Base(checksumPath))
	}
	version, valid := strings.CutPrefix(releaseName, "terraform-provider-openai_")
	if !valid || version == "" {
		return fmt.Errorf("checksum manifest %q has an unsupported provider release identity", filepath.Base(checksumPath))
	}
	registryName := releaseName + "_manifest.json"
	registryPath := filepath.Join(filepath.Dir(filepath.Dir(checksumPath)), "terraform-registry-manifest.json")
	registry, err := verifyMetadataArtifact(registryName, registryPath, checksums, verifyRegistryManifest)
	if err != nil {
		return err
	}
	*metadata = append(*metadata, registry)

	for name, path := range archives {
		if err := verifyReleasePlatform(name, releaseName); err != nil {
			return err
		}
		sbomName := name + ".spdx.json"
		sbomPath, exists := sboms[sbomName]
		if !exists || sbomPath != path+".spdx.json" {
			return fmt.Errorf("archive %q has no matching adjacent SBOM", name)
		}
		if err := verifyArtifact(name, path, checksums); err != nil {
			return err
		}
		sbom, err := verifyMetadataArtifact(sbomName, sbomPath, checksums, verifySPDX)
		if err != nil {
			return err
		}
		*metadata = append(*metadata, sbom)
	}

	for name := range checksums {
		switch {
		case strings.HasSuffix(name, ".zip"):
			if _, exists := archives[name]; !exists {
				return fmt.Errorf("checksum manifest lists missing archive %q", name)
			}
		case strings.HasSuffix(name, ".spdx.json"):
			if _, exists := sboms[name]; !exists {
				return fmt.Errorf("checksum manifest lists missing SBOM %q", name)
			}
		case name == registryName:
		default:
			return fmt.Errorf("unexpected checksum manifest entry %q", name)
		}
	}
	return nil
}

func readArtifactSnapshot(path string) ([]byte, fs.FileMode, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read release artifact snapshot: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("inspect release artifact snapshot: %w", statErr)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, 0, errors.New("release artifact snapshot is not a regular file")
	}
	manifest, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, 0, fmt.Errorf("read release artifact snapshot: %w", readErr)
	}
	if closeErr != nil {
		return nil, 0, fmt.Errorf("close release artifact snapshot: %w", closeErr)
	}
	return manifest, info.Mode().Perm(), nil
}

func readChecksums(manifest []byte) (map[string][]byte, error) {
	checksums := make(map[string][]byte)
	scanner := bufio.NewScanner(bytes.NewReader(manifest))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < sha256.Size*2+3 || line[sha256.Size*2] != ' ' ||
			line[sha256.Size*2+1] != ' ' && line[sha256.Size*2+1] != '*' {
			return nil, fmt.Errorf("malformed SHA-256 checksum manifest entry %q", line)
		}

		name := line[sha256.Size*2+2:]
		if filepath.Base(name) != name || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("checksum manifest entry %q is not an artifact filename", name)
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("duplicate checksum manifest entry for %q", name)
		}

		digest, err := hex.DecodeString(line[:sha256.Size*2])
		if err != nil {
			return nil, fmt.Errorf("invalid SHA-256 digest for %q: %w", name, err)
		}
		checksums[name] = digest
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksum manifest: %w", err)
	}
	return checksums, nil
}

func publishVerifiedSnapshot(path string, contents []byte, mode fs.FileMode) error {
	snapshot, err := os.CreateTemp(filepath.Dir(path), ".verified-artifact-*")
	if err != nil {
		return fmt.Errorf("create verified artifact snapshot: %w", err)
	}
	defer func() {
		_ = os.Remove(snapshot.Name())
	}()
	if err := snapshot.Chmod(mode); err != nil {
		_ = snapshot.Close()
		return fmt.Errorf("set verified artifact permissions: %w", err)
	}
	_, writeErr := snapshot.Write(contents)
	closeErr := snapshot.Close()
	if writeErr != nil {
		return fmt.Errorf("write verified artifact snapshot: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close verified artifact snapshot: %w", closeErr)
	}
	if err := os.Rename(snapshot.Name(), path); err != nil {
		return fmt.Errorf("publish verified artifact snapshot: %w", err)
	}
	return nil
}

func verifyReleasePlatform(name, releaseName string) error {
	archive, valid := strings.CutSuffix(name, ".zip")
	if !valid {
		return fmt.Errorf("archive %q is not a ZIP release artifact", name)
	}
	platform, valid := strings.CutPrefix(archive, releaseName+"_")
	if !valid {
		return fmt.Errorf("archive %q does not belong to release %q", name, releaseName)
	}
	switch platform {
	case "darwin_amd64", "darwin_arm64",
		"freebsd_386", "freebsd_amd64", "freebsd_arm", "freebsd_arm64",
		"linux_386", "linux_amd64", "linux_arm", "linux_arm64",
		"windows_386", "windows_amd64", "windows_arm64":
		return nil
	default:
		return fmt.Errorf("archive %q has unsupported release platform %q", name, platform)
	}
}

func verifyArtifact(name, path string, checksums map[string][]byte) error {
	expected, exists := checksums[name]
	if !exists {
		return fmt.Errorf("checksum manifest has no entry for %q", name)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open release artifact %q: %w", name, err)
	}

	hash := sha256.New()
	_, hashErr := io.Copy(hash, file)
	closeErr := file.Close()
	if hashErr != nil {
		return fmt.Errorf("hash release artifact %q: %w", name, hashErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close release artifact %q: %w", name, closeErr)
	}
	if !bytes.Equal(hash.Sum(nil), expected) {
		return fmt.Errorf("SHA-256 checksum mismatch for %q", name)
	}
	return nil
}

type verifiedArtifact struct {
	name     string
	path     string
	contents []byte
	mode     fs.FileMode
}

func verifyMetadataArtifact(name, path string, checksums map[string][]byte, validate func(string, []byte) error) (verifiedArtifact, error) {
	expected, exists := checksums[name]
	if !exists {
		return verifiedArtifact{}, fmt.Errorf("checksum manifest has no entry for %q", name)
	}
	contents, mode, err := readArtifactSnapshot(path)
	if err != nil {
		return verifiedArtifact{}, fmt.Errorf("open release artifact %q: %w", name, err)
	}
	digest := sha256.Sum256(contents)
	if !bytes.Equal(digest[:], expected) {
		return verifiedArtifact{}, fmt.Errorf("SHA-256 checksum mismatch for %q", name)
	}
	if err := validate(path, contents); err != nil {
		return verifiedArtifact{}, err
	}
	return verifiedArtifact{name: name, path: path, contents: contents, mode: mode}, nil
}

func verifySPDX(path string, document []byte) error {
	var spdx struct {
		Version  string            `json:"spdxVersion"`
		Packages []json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(document, &spdx); err != nil {
		return fmt.Errorf("parse SPDX SBOM %q: %w", path, err)
	}
	if spdx.Version == "" || len(spdx.Packages) == 0 {
		return fmt.Errorf("SPDX SBOM %q has no version or packages", path)
	}
	return nil
}

func verifyRegistryManifest(path string, document []byte) error {
	var manifest struct {
		Version  int `json:"version"`
		Metadata struct {
			ProtocolVersions []string `json:"protocol_versions"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(document, &manifest); err != nil {
		return fmt.Errorf("parse Terraform Registry manifest %q: %w", path, err)
	}
	if manifest.Version != 1 {
		return fmt.Errorf("terraform registry manifest %q has unsupported version %d", path, manifest.Version)
	}
	if len(manifest.Metadata.ProtocolVersions) == 0 {
		return fmt.Errorf("terraform registry manifest %q has no protocol versions", path)
	}

	versions := make(map[string]struct{}, len(manifest.Metadata.ProtocolVersions))
	for _, version := range manifest.Metadata.ProtocolVersions {
		major, minor, valid := strings.Cut(version, ".")
		if !valid || major == "" || minor == "" || strings.ContainsFunc(major+minor, func(character rune) bool {
			return character < '0' || character > '9'
		}) {
			return fmt.Errorf("terraform registry manifest %q has invalid protocol version %q", path, version)
		}
		if _, exists := versions[version]; exists {
			return fmt.Errorf("terraform registry manifest %q has duplicate protocol version %q", path, version)
		}
		versions[version] = struct{}{}
	}
	return nil
}
