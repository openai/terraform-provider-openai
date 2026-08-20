package repopolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dependabotMaturityCooldown = "\n    cooldown:\n      default-days: 7\n"

func TestDependabotVersionUpdatesUseMaturityCooldown(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", ".github", "dependabot.yml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Dependabot configuration: %v", err)
	}

	updates := dependabotUpdateBlocks(string(contents))
	wantUpdates := map[string]bool{
		"gomod:/":          false,
		"gomod:/tools":     false,
		"github-actions:/": false,
	}
	if len(updates) != len(wantUpdates) {
		t.Fatalf("found %d Dependabot update blocks, want %d", len(updates), len(wantUpdates))
	}

	for _, update := range updates {
		ecosystem, err := quotedYAMLValue(update, "package-ecosystem")
		if err != nil {
			t.Fatal(err)
		}
		directory, err := quotedYAMLValue(update, "directory")
		if err != nil {
			t.Fatal(err)
		}

		key := ecosystem + ":" + directory
		seen, expected := wantUpdates[key]
		if !expected {
			t.Fatalf("unexpected Dependabot update block %q", key)
		}
		if seen {
			t.Fatalf("duplicate Dependabot update block %q", key)
		}
		wantUpdates[key] = true

		if !strings.Contains(update, dependabotMaturityCooldown) {
			t.Errorf("Dependabot update block %q must use a seven-day maturity cooldown", key)
		}
	}

	for key, seen := range wantUpdates {
		if !seen {
			t.Errorf("missing Dependabot update block %q", key)
		}
	}
}

func dependabotUpdateBlocks(config string) []string {
	const marker = "  - package-ecosystem:"

	parts := strings.Split(config, marker)
	updates := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		updates = append(updates, marker+part)
	}

	return updates
}

func quotedYAMLValue(block, key string) (string, error) {
	prefix := key + ":"
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			return "", fmt.Errorf("Dependabot %s must be a quoted string, got %q", key, value)
		}
		return value[1 : len(value)-1], nil
	}

	return "", fmt.Errorf("Dependabot update block is missing %s", key)
}
