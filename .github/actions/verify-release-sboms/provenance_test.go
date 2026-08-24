package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestProductionReleaseRequiresVerifiedArtifactProvenance(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}

	workflow := string(contents)
	publishStart := strings.Index(workflow, "\n  goreleaser:\n")
	if publishStart < 0 {
		t.Fatal("production publishing job was not found")
	}
	preflight, publish := workflow[:publishStart], workflow[publishStart:]

	t.Run("protected publishing job owns attestation permissions", func(t *testing.T) {
		if strings.Contains(preflight, "id-token: write") || strings.Contains(preflight, "attestations: write") {
			t.Fatal("release preflight can issue OIDC tokens or create artifact attestations")
		}

		permissions := regexp.MustCompile(`(?m)^    environment: publish\n    permissions:\n      attestations: write\n      contents: write\n      id-token: write\n`)
		if !permissions.MatchString(publish) {
			t.Fatal("approved publishing job does not exclusively own its required release and provenance permissions")
		}
	})

	t.Run("signed release inventory is attested before publication", func(t *testing.T) {
		attestation := regexp.MustCompile(`(?m)^      - name: Attest verified release artifacts\n        uses: actions/attest@[0-9a-f]{40}(?:[ \t]+#[^\n]*)?\n`)
		if !attestation.MatchString(publish) {
			t.Fatal("release provenance action is not pinned to an immutable full commit SHA")
		}

		steps := []string{
			"      - name: Run GoReleaser\n        id: goreleaser\n",
			`"$RELEASE_TOOLS_DIR/goreleaser" release --clean --draft`,
			`echo "checksums=dist/terraform-provider-openai_${GITHUB_REF_NAME#v}_SHA256SUMS" >> "$GITHUB_OUTPUT"`,
			"      - name: Attest verified release artifacts\n",
			"          subject-checksums: ${{ steps.goreleaser.outputs.checksums }}\n",
			"      - name: Verify provider archive attestations\n",
			"          for archive in dist/*.zip; do\n",
			`gh attestation verify "$archive"`,
			`--repo "$GITHUB_REPOSITORY"`,
			`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yml"`,
			`--source-ref "$GITHUB_REF"`,
			`--source-digest "$GITHUB_SHA"`,
			"      - name: Verify and publish release draft\n",
		}

		remaining := publish
		for _, step := range steps {
			index := strings.Index(remaining, step)
			if index < 0 {
				t.Fatalf("production release omits or misorders provenance requirement %q", step)
			}
			remaining = remaining[index+len(step):]
		}
	})
}
