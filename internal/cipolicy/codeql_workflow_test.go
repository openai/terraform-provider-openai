package cipolicy_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCodeQLBuildCoversGoSourcesFromCleanCache(t *testing.T) {
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

	const expectedBuildStep = `      - name: Build Go source for CodeQL
        if: matrix.language == 'go'
        run: |
          go clean -cache
          go build -buildvcs=false -mod=readonly ./...
          go test -buildvcs=false -mod=readonly -run '^$' ./...
          go test -buildvcs=false -mod=readonly -run '^$' ./.github/actions/setup-terraform`
	if !strings.Contains(string(workflow), expectedBuildStep) {
		t.Fatalf(
			"CodeQL Go build must clear restored build artifacts before compiling every active source:\n%s",
			expectedBuildStep,
		)
	}
}
