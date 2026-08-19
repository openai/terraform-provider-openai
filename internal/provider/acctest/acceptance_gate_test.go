package acctest

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEnforceExactAcceptanceOptIn(t *testing.T) {
	testCases := []struct {
		name    string
		value   string
		exists  bool
		enabled bool
	}{
		{name: "unset"},
		{name: "blank", exists: true},
		{name: "whitespace", value: "  ", exists: true},
		{name: "zero", value: "0", exists: true},
		{name: "false", value: "false", exists: true},
		{name: "true", value: "true", exists: true},
		{name: "padded one", value: " 1 ", exists: true},
		{name: "exact one", value: "1", exists: true, enabled: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("TF_ACC", testCase.value)
			if !testCase.exists {
				if err := os.Unsetenv("TF_ACC"); err != nil {
					t.Fatal(err)
				}
			}

			if err := enforceExactAcceptanceOptIn(os.LookupEnv, os.Unsetenv); err != nil {
				t.Fatal(err)
			}

			value, exists := os.LookupEnv("TF_ACC")
			if exists != testCase.enabled || (exists && value != "1") {
				t.Fatalf("acceptance opt-in state = (%q, %t); enabled = %t", value, exists, testCase.enabled)
			}
		})
	}
}

func TestEnforceExactAcceptanceOptInFailsClosed(t *testing.T) {
	want := errors.New("synthetic unset failure")
	err := enforceExactAcceptanceOptIn(
		func(key string) (string, bool) {
			if key != "TF_ACC" {
				t.Fatalf("unexpected environment lookup: %q", key)
			}
			return "0", true
		},
		func(key string) error {
			if key != "TF_ACC" {
				t.Fatalf("unexpected environment removal: %q", key)
			}
			return want
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("unset failure = %v; want %v", err, want)
	}
}

func TestAcceptanceOptInIsSanitizedDuringPackageInitialization(t *testing.T) {
	if os.Getenv("OPENAI_ACCEPTANCE_GATE_TEST_SUBPROCESS") == "1" {
		value, exists := os.LookupEnv("TF_ACC")
		enabled := os.Getenv("OPENAI_ACCEPTANCE_GATE_EXPECT_ENABLED") == "1"
		if exists != enabled || (exists && value != "1") {
			t.Fatalf("initialized acceptance opt-in = (%q, %t); enabled = %t", value, exists, enabled)
		}
		return
	}

	testCases := []struct {
		name    string
		value   string
		exists  bool
		enabled bool
	}{
		{name: "unset"},
		{name: "blank", exists: true},
		{name: "whitespace", value: "  ", exists: true},
		{name: "zero", value: "0", exists: true},
		{name: "false", value: "false", exists: true},
		{name: "padded one", value: " 1 ", exists: true},
		{name: "exact one", value: "1", exists: true, enabled: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			command := exec.Command(
				os.Args[0],
				"-test.run=^TestAcceptanceOptInIsSanitizedDuringPackageInitialization$",
			)
			command.Env = []string{"OPENAI_ACCEPTANCE_GATE_TEST_SUBPROCESS=1"}
			if testCase.exists {
				command.Env = append(command.Env, "TF_ACC="+testCase.value)
			}
			if testCase.enabled {
				command.Env = append(command.Env, "OPENAI_ACCEPTANCE_GATE_EXPECT_ENABLED=1")
			}
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("credential-free initialization subprocess failed: %v\n%s", err, output)
			}
		})
	}
}

func TestGeneratedAcceptancePackagesImportSharedGuard(t *testing.T) {
	const sharedGuard = "github.com/openai/terraform-provider-openai/internal/provider/acctest"

	count := 0
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_acc_test.go") {
			return nil
		}

		count++
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if importPath == sharedGuard {
				return nil
			}
		}

		t.Errorf("generated acceptance test bypasses shared opt-in guard: %s", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no generated acceptance tests were checked")
	}
	t.Logf("verified %d generated acceptance test files import the shared guard", count)
}
