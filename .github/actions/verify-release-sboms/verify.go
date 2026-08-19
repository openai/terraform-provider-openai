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

	if err := verifyRelease(args[0]); err != nil {
		return err
	}
	if len(args) == 1 {
		return nil
	}

	sign := exec.Command("gpg", args[2:]...)
	sign.Stdin = os.Stdin
	sign.Stdout = os.Stdout
	sign.Stderr = os.Stderr
	if err := sign.Run(); err != nil {
		return fmt.Errorf("sign verified checksum manifest: %w", err)
	}
	return nil
}

func verifyRelease(checksumPath string) error {
	checksums, err := readChecksums(checksumPath)
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

	for name, path := range archives {
		sbomName := name + ".spdx.json"
		sbomPath, exists := sboms[sbomName]
		if !exists || sbomPath != path+".spdx.json" {
			return fmt.Errorf("archive %q has no matching adjacent SBOM", name)
		}
		if err := verifyArtifact(name, path, checksums); err != nil {
			return err
		}
		if err := verifyArtifact(sbomName, sbomPath, checksums); err != nil {
			return err
		}
		if err := verifySPDX(sbomPath); err != nil {
			return err
		}
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
		}
	}
	return nil
}

func readChecksums(path string) (map[string][]byte, error) {
	manifest, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read checksum manifest: %w", err)
	}

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

func verifySPDX(path string) error {
	document, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read SBOM %q: %w", path, err)
	}

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
