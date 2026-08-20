package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRejectsOmittedConfiguredPlatforms(t *testing.T) {
	t.Parallel()

	for _, platform := range releasePlatforms {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			fixture := newReleaseFixture(t)
			archive := filepath.Join(fixture.directory, "terraform-provider-openai_1.2.3_"+platform+".zip")
			for _, path := range []string{archive, archive + ".spdx.json"} {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			fixture.writeChecksums(t)
			err := verifyRelease(fixture.manifest)
			if err == nil || !strings.Contains(err.Error(), "missing configured platform \""+platform+"\"") {
				t.Fatalf("incomplete release verification error = %v, want missing configured platform %q", err, platform)
			}
		})
	}
}
