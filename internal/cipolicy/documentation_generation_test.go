package cipolicy

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	testDocsModule  = "example.com/tfplugindocs"
	testDocsVersion = "v0.0.1"
)

func writeDocumentationTestFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", path, err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeDocumentationTestProxy(t *testing.T, root string) string {
	t.Helper()

	versionDir := filepath.Join(root, "proxy", "example.com", "tfplugindocs", "@v")
	writeDocumentationTestFile(t, filepath.Join(versionDir, "list"), []byte(testDocsVersion+"\n"))
	writeDocumentationTestFile(
		t,
		filepath.Join(versionDir, testDocsVersion+".info"),
		[]byte(`{"Version":"v0.0.1","Time":"2026-01-01T00:00:00Z"}`),
	)
	moduleFile := []byte("module " + testDocsModule + "\n\ngo 1.25.0\n")
	writeDocumentationTestFile(t, filepath.Join(versionDir, testDocsVersion+".mod"), moduleFile)

	archivePath := filepath.Join(versionDir, testDocsVersion+".zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create module archive: %v", err)
	}
	t.Cleanup(func() { _ = archiveFile.Close() })
	archive := zip.NewWriter(archiveFile)
	prefix := testDocsModule + "@" + testDocsVersion + "/"
	for name, contents := range map[string][]byte{
		"go.mod":  moduleFile,
		"main.go": []byte("package main\n\nfunc main() {}\n"),
	} {
		entry, createErr := archive.Create(prefix + name)
		if createErr != nil {
			t.Fatalf("create %s archive entry: %v", name, createErr)
		}
		if _, writeErr := entry.Write(contents); writeErr != nil {
			t.Fatalf("write %s archive entry: %v", name, writeErr)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close module archive: %v", err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatalf("close module archive file: %v", err)
	}

	return "file://" + filepath.ToSlash(filepath.Join(root, "proxy"))
}

func TestDocumentationGenerationRejectsMissingToolChecksumsWithoutMutation(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	makefile, err := os.ReadFile(filepath.Join(repositoryRoot, "GNUmakefile"))
	if err != nil {
		t.Fatalf("read GNUmakefile: %v", err)
	}
	generateCommand := regexp.MustCompile(`(?m)^\tcd tools; GOWORK=off go generate -mod=(mod|readonly) \./\.\.\.$`).FindSubmatch(makefile)
	if generateCommand == nil {
		t.Fatal("GNUmakefile generate command did not match the governed form")
	}

	toolsSource, err := os.ReadFile(filepath.Join(repositoryRoot, "tools", "tools.go"))
	if err != nil {
		t.Fatalf("read tools/tools.go: %v", err)
	}
	rewrittenTools := strings.ReplaceAll(
		string(toolsSource),
		"github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs",
		testDocsModule,
	)
	if rewrittenTools == string(toolsSource) {
		t.Fatal("tools/tools.go did not contain the documentation generator target")
	}

	testRoot := t.TempDir()
	toolsDir := filepath.Join(testRoot, "tools")
	writeDocumentationTestFile(
		t,
		filepath.Join(toolsDir, "go.mod"),
		[]byte("module example.com/provider-tools\n\ngo 1.25.0\n\nrequire "+testDocsModule+" "+testDocsVersion+"\n"),
	)
	writeDocumentationTestFile(t, filepath.Join(toolsDir, "go.sum"), nil)
	writeDocumentationTestFile(t, filepath.Join(toolsDir, "tools.go"), []byte(rewrittenTools))

	moduleBefore, err := os.ReadFile(filepath.Join(toolsDir, "go.mod"))
	if err != nil {
		t.Fatalf("read test go.mod: %v", err)
	}
	checksumsBefore, err := os.ReadFile(filepath.Join(toolsDir, "go.sum"))
	if err != nil {
		t.Fatalf("read test go.sum: %v", err)
	}

	command := exec.Command("go", "generate", "-mod="+string(generateCommand[1]), "./...")
	command.Dir = toolsDir
	command.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOCACHE="+filepath.Join(testRoot, "build-cache"),
		"GOENV=off",
		"GOFLAGS=-buildvcs=false -modcacherw",
		"GOMODCACHE="+filepath.Join(testRoot, "module-cache"),
		"GONOPROXY=none",
		"GONOSUMDB=*",
		"GOPROXY="+writeDocumentationTestProxy(t, testRoot),
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	)
	output, runErr := command.CombinedOutput()
	if runErr == nil {
		t.Errorf("documentation generation succeeded with a missing tool checksum; output: %s", output)
	}

	moduleAfter, err := os.ReadFile(filepath.Join(toolsDir, "go.mod"))
	if err != nil {
		t.Fatalf("read resulting go.mod: %v", err)
	}
	checksumsAfter, err := os.ReadFile(filepath.Join(toolsDir, "go.sum"))
	if err != nil {
		t.Fatalf("read resulting go.sum: %v", err)
	}
	if string(moduleAfter) != string(moduleBefore) {
		t.Error("documentation generation mutated tools/go.mod")
	}
	if string(checksumsAfter) != string(checksumsBefore) {
		t.Error("documentation generation mutated tools/go.sum")
	}
	if mode := string(generateCommand[1]); mode != "readonly" {
		t.Errorf("GNUmakefile documentation generation mode = %q, want readonly", mode)
	}
}
