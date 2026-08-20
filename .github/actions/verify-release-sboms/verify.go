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
	if len(args) > 0 && args[0] == "--publish" {
		if len(args) != 3 {
			return errors.New("usage: verify.go --publish RELEASE_TAG SIGNING_FINGERPRINT")
		}
		return publishReleaseDraft(args[1], args[2])
	}
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
	var artifacts []verifiedArtifact
	if err := verifyReleaseSnapshot(args[0], manifest, &artifacts); err != nil {
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
	for _, artifact := range artifacts {
		if err := publishVerifiedSnapshot(artifact.path, artifact.contents, artifact.mode); err != nil {
			return fmt.Errorf("publish verified release artifact %q: %w", artifact.name, err)
		}
	}
	return publishVerifiedSnapshot(args[0], manifest, mode)
}

func verifyRelease(checksumPath string) error {
	return run([]string{checksumPath})
}

func verifyReleaseSnapshot(checksumPath string, manifest []byte, artifacts *[]verifiedArtifact) error {
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
	*artifacts = append(*artifacts, registry)

	for name, path := range archives {
		if err := verifyReleasePlatform(name, releaseName); err != nil {
			return err
		}
		sbomName := name + ".spdx.json"
		sbomPath, exists := sboms[sbomName]
		if !exists || sbomPath != path+".spdx.json" {
			return fmt.Errorf("archive %q has no matching adjacent SBOM", name)
		}
		archive, err := verifyArtifact(name, path, checksums)
		if err != nil {
			return err
		}
		*artifacts = append(*artifacts, archive)
		sbom, err := verifyMetadataArtifact(sbomName, sbomPath, checksums, verifySPDX)
		if err != nil {
			return err
		}
		*artifacts = append(*artifacts, sbom)
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
	return verifyCompleteReleasePlatforms(releaseName, func(name string) bool {
		_, exists := archives[name]
		return exists
	})
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

var releasePlatforms = [...]string{
	"darwin_amd64", "darwin_arm64",
	"freebsd_386", "freebsd_amd64", "freebsd_arm", "freebsd_arm64",
	"linux_386", "linux_amd64", "linux_arm", "linux_arm64",
	"windows_386", "windows_amd64", "windows_arm64",
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
	for _, supported := range releasePlatforms {
		if platform == supported {
			return nil
		}
	}
	return fmt.Errorf("archive %q has unsupported release platform %q", name, platform)
}

func verifyCompleteReleasePlatforms(releaseName string, exists func(string) bool) error {
	for _, platform := range releasePlatforms {
		if !exists(releaseName + "_" + platform + ".zip") {
			return fmt.Errorf("release is missing configured platform %q", platform)
		}
	}
	return nil
}

type verifiedArtifact struct {
	name     string
	path     string
	contents []byte
	mode     fs.FileMode
}

func verifyArtifact(name, path string, checksums map[string][]byte) (verifiedArtifact, error) {
	contents, mode, err := readArtifactSnapshot(path)
	if err != nil {
		return verifiedArtifact{}, fmt.Errorf("open release artifact %q: %w", name, err)
	}
	if err := verifyArtifactContents(name, contents, checksums); err != nil {
		return verifiedArtifact{}, err
	}
	return verifiedArtifact{name: name, path: path, contents: contents, mode: mode}, nil
}

func verifyArtifactContents(name string, contents []byte, checksums map[string][]byte) error {
	expected, exists := checksums[name]
	if !exists {
		return fmt.Errorf("checksum manifest has no entry for %q", name)
	}
	digest := sha256.Sum256(contents)
	if !bytes.Equal(digest[:], expected) {
		return fmt.Errorf("SHA-256 checksum mismatch for %q", name)
	}
	return nil
}

func verifyMetadataArtifact(name, path string, checksums map[string][]byte, validate func(string, []byte) error) (verifiedArtifact, error) {
	artifact, err := verifyArtifact(name, path, checksums)
	if err != nil {
		return verifiedArtifact{}, err
	}
	if err := validate(path, artifact.contents); err != nil {
		return verifiedArtifact{}, err
	}
	return artifact, nil
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

func publishReleaseDraft(tag, fingerprint string) error {
	assets, err := readDraftAssets(tag)
	if err != nil {
		return err
	}
	releaseName := "terraform-provider-openai_" + strings.TrimPrefix(tag, "v")
	checksumName := releaseName + "_SHA256SUMS"
	signatureName := checksumName + ".sig"
	for _, name := range []string{checksumName, signatureName} {
		if _, exists := assets[name]; !exists {
			return fmt.Errorf("missing signed checksum artifact %q", name)
		}
	}

	manifest, err := downloadReleaseAsset(tag, assets[checksumName])
	if err != nil {
		return err
	}
	signature, err := downloadReleaseAsset(tag, assets[signatureName])
	if err != nil {
		return err
	}
	if err := verifyReleaseSignature(manifest, signature, fingerprint); err != nil {
		return err
	}
	checksums, err := readChecksums(manifest)
	if err != nil {
		return err
	}
	if err := verifyDraftArtifacts(tag, releaseName, assets, checksums); err != nil {
		return err
	}
	current, err := readDraftAssets(tag)
	if err != nil {
		return err
	}
	if len(current) != len(assets) {
		return errors.New("uploaded release assets changed during verification")
	}
	for name, asset := range assets {
		if current[name] != asset {
			return errors.New("uploaded release assets changed during verification")
		}
	}
	if output, err := exec.Command("gh", "release", "edit", tag, "--draft=false").CombinedOutput(); err != nil {
		return fmt.Errorf("publish verified release draft: %w: %s", err, bytes.TrimSpace(output))
	}
	return nil
}

type releaseAsset struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
	State  string `json:"state"`
}

func readDraftAssets(tag string) (map[string]releaseAsset, error) {
	output, err := exec.Command("gh", "release", "view", tag, "--json", "tagName,isDraft,assets").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect private release draft: %w", err)
	}
	var release struct {
		TagName string         `json:"tagName"`
		Draft   bool           `json:"isDraft"`
		Assets  []releaseAsset `json:"assets"`
	}
	if err := json.Unmarshal(output, &release); err != nil {
		return nil, fmt.Errorf("parse private release draft: %w", err)
	}
	if release.TagName != tag || !release.Draft {
		return nil, fmt.Errorf("release %q is not a private draft for the requested tag", tag)
	}

	assets := make(map[string]releaseAsset, len(release.Assets))
	for _, asset := range release.Assets {
		if filepath.Base(asset.Name) != asset.Name || strings.Contains(asset.Name, "\\") {
			return nil, fmt.Errorf("uploaded release asset %q is not an artifact filename", asset.Name)
		}
		if _, exists := assets[asset.Name]; exists {
			return nil, fmt.Errorf("duplicate uploaded release artifact %q", asset.Name)
		}
		digest, valid := strings.CutPrefix(asset.Digest, "sha256:")
		decoded, err := hex.DecodeString(digest)
		if asset.ID == "" || asset.State != "uploaded" || !valid || err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("uploaded release artifact %q has no immutable uploaded identity and SHA-256 digest", asset.Name)
		}
		assets[asset.Name] = asset
	}
	return assets, nil
}

func verifyDraftArtifacts(tag, releaseName string, assets map[string]releaseAsset, checksums map[string][]byte) error {
	registryName := releaseName + "_manifest.json"
	checksumName := releaseName + "_SHA256SUMS"
	signatureName := checksumName + ".sig"
	archives := 0
	sboms := 0
	for name, asset := range assets {
		if name == checksumName || name == signatureName {
			continue
		}
		if name != registryName && !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".spdx.json") {
			return fmt.Errorf("unexpected uploaded release artifact %q", name)
		}
		contents, err := downloadReleaseAsset(tag, asset)
		if err != nil {
			return err
		}
		if err := verifyArtifactContents(name, contents, checksums); err != nil {
			return err
		}
		switch {
		case name == registryName:
			if err := verifyRegistryManifest(name, contents); err != nil {
				return err
			}
		case strings.HasSuffix(name, ".zip"):
			if err := verifyReleasePlatform(name, releaseName); err != nil {
				return err
			}
			if _, exists := assets[name+".spdx.json"]; !exists {
				return fmt.Errorf("uploaded archive %q has no matching SPDX SBOM", name)
			}
			archives++
		case strings.HasSuffix(name, ".spdx.json"):
			if err := verifySPDX(name, contents); err != nil {
				return err
			}
			if _, exists := assets[strings.TrimSuffix(name, ".spdx.json")]; !exists {
				return fmt.Errorf("uploaded SPDX SBOM %q has no matching archive", name)
			}
			sboms++
		default:
			return fmt.Errorf("unexpected uploaded release artifact %q", name)
		}
	}
	if _, exists := assets[registryName]; !exists {
		return fmt.Errorf("uploaded release has no Terraform Registry manifest %q", registryName)
	}
	if archives == 0 || archives != sboms || len(checksums) != len(assets)-2 {
		return fmt.Errorf("uploaded release has an incomplete signed artifact set: archives=%d SBOMs=%d checksums=%d", archives, sboms, len(checksums))
	}
	return verifyCompleteReleasePlatforms(releaseName, func(name string) bool {
		_, exists := assets[name]
		return exists
	})
}

func downloadReleaseAsset(tag string, asset releaseAsset) ([]byte, error) {
	contents, err := exec.Command("gh", "release", "download", tag, "--pattern", asset.Name, "--output", "-").Output()
	if err != nil {
		return nil, fmt.Errorf("download uploaded release artifact %q: %w", asset.Name, err)
	}
	if fmt.Sprintf("sha256:%x", sha256.Sum256(contents)) != asset.Digest {
		return nil, fmt.Errorf("uploaded release artifact %q does not match its immutable SHA-256 digest", asset.Name)
	}
	return contents, nil
}

func verifyReleaseSignature(manifest, signature []byte, fingerprint string) error {
	file, err := os.CreateTemp("", "verified-release-signature-*")
	if err != nil {
		return fmt.Errorf("create uploaded signature snapshot: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	if _, err := file.Write(signature); err != nil {
		return fmt.Errorf("write uploaded signature snapshot: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind uploaded signature snapshot: %w", err)
	}
	if err := os.Remove(file.Name()); err != nil {
		return fmt.Errorf("seal uploaded signature snapshot: %w", err)
	}
	verify := exec.Command("gpg", "--batch", "--assert-signer", fingerprint, "--verify", "/dev/fd/3", "-")
	verify.ExtraFiles = []*os.File{file}
	verify.Stdin = bytes.NewReader(manifest)
	if output, err := verify.CombinedOutput(); err != nil {
		return fmt.Errorf("verify uploaded checksum signature: %w: %s", err, bytes.TrimSpace(output))
	}
	return nil
}
