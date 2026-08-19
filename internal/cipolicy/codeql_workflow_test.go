package cipolicy_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCodeQLBuildCoversHiddenTerraformInstaller(t *testing.T) {
	t.Parallel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the CodeQL workflow regression test")
	}

	workflowPath := filepath.Join(
		filepath.Dir(sourceFile),
		"..", "..", ".github", "workflows", "codeql.yml",
	)
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read CodeQL workflow: %v", err)
	}

	const (
		buildStep = "- name: Build Go source for CodeQL"
		command   = "go test -buildvcs=false -mod=readonly -run '^$' ./.github/actions/setup-terraform"
	)

	inBuildStep := false
	for line := range strings.Lines(string(workflow)) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- name: ") {
			inBuildStep = trimmed == buildStep
			continue
		}
		if inBuildStep && trimmed == command {
			return
		}
	}

	t.Fatalf(
		"%q must compile-test the dot-prefixed Terraform installer package with %q",
		buildStep,
		command,
	)
}
