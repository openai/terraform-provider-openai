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
	publish := workflow[publishStart:]

	t.Run("protected publishing job owns attestation permissions", func(t *testing.T) {
		if !hasExclusiveReleaseProvenancePermissions(workflow) {
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

func TestReleaseProvenanceRejectsUnprotectedPermissionGrants(t *testing.T) {
	t.Parallel()

	protected := "jobs:\n" +
		"  goreleaser:\n" +
		"    environment: publish\n" +
		"    permissions:\n" +
		"      attestations: write\n" +
		"      contents: write\n" +
		"      id-token: write\n"
	unprotected := "  unapproved:\n" +
		"    permissions:\n" +
		"      attestations: write\n" +
		"      id-token: write\n"

	tests := []struct {
		name     string
		workflow string
		allowed  bool
	}{
		{name: "only protected publishing job", workflow: protected, allowed: true},
		{
			name:     "workflow-wide OIDC permission",
			workflow: "permissions:\n  id-token: write\n" + protected,
		},
		{
			name:     "workflow-wide attestation permission",
			workflow: "permissions:\n  attestations: write\n" + protected,
		},
		{
			name:     "workflow-wide write-all permissions",
			workflow: "permissions: write-all\n" + protected,
		},
		{
			name:     "quoted workflow-wide write-all permissions",
			workflow: "permissions: 'write-all' # all workflow permissions\n" + protected,
		},
		{
			name:     "quoted workflow-wide write-all permission key",
			workflow: "\"permissions\": \"write-all\"\n" + protected,
		},
		{
			name:     "unapproved preceding job",
			workflow: strings.Replace(protected, "  goreleaser:\n", unprotected+"  goreleaser:\n", 1),
		},
		{
			name: "unapproved preceding write-all job",
			workflow: strings.Replace(protected, "  goreleaser:\n",
				"  unapproved:\n    permissions: write-all\n  goreleaser:\n", 1),
		},
		{
			name:     "unapproved following job",
			workflow: protected + unprotected,
		},
		{
			name: "unapproved following job with aligned permissions",
			workflow: protected + "  unapproved:\n" +
				"    permissions:\n" +
				"      id-token:    write\n" +
				"      attestations:    write\n",
		},
		{
			name: "unapproved following job with quoted permissions",
			workflow: protected + "  unapproved:\n" +
				"    permissions:\n" +
				"      'id-token': \"write\"\n" +
				"      \"attestations\": 'write'\n",
		},
		{
			name: "unapproved following job with inline permissions",
			workflow: protected + "  unapproved:\n" +
				"    permissions: {id-token: write, attestations: write}\n",
		},
		{
			name:     "unapproved following write-all job",
			workflow: protected + "  unapproved:\n    permissions: write-all\n",
		},
		{
			name: "unapproved following write-all job with quoted permission key",
			workflow: protected + "  unapproved:\n" +
				"    'permissions': 'write-all'\n",
		},
		{
			name:     "unprotected publishing environment",
			workflow: strings.Replace(protected, "environment: publish", "environment: staging", 1),
		},
		{
			name:     "missing OIDC permission",
			workflow: strings.Replace(protected, "id-token: write", "id-token: read", 1),
		},
		{
			name:     "missing attestation permission",
			workflow: strings.Replace(protected, "attestations: write", "attestations: read", 1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if allowed := hasExclusiveReleaseProvenancePermissions(test.workflow); allowed != test.allowed {
				t.Fatalf("release provenance permissions allowed = %t, want %t", allowed, test.allowed)
			}
		})
	}
}

func hasExclusiveReleaseProvenancePermissions(workflow string) bool {
	writeAll := regexp.MustCompile(`(?m)^[ \t]*["']?permissions["']?[ \t]*:[ \t]*["']?write-all["']?[ \t]*(?:#.*)?$`)
	if writeAll.MatchString(workflow) {
		return false
	}

	for _, permission := range []string{"id-token", "attestations"} {
		grant := regexp.MustCompile(`(?m)(?:^|[,{])[ \t]*["']?` + regexp.QuoteMeta(permission) +
			`["']?[ \t]*:[ \t]*["']?write["']?[ \t]*(?:[,}]|#.*|$)`)
		if len(grant.FindAllStringIndex(workflow, -1)) != 1 {
			return false
		}
	}

	protected := regexp.MustCompile(`(?m)^    environment: publish\n    permissions:\n      attestations: write\n      contents: write\n      id-token: write\n`)
	return protected.MatchString(workflow)
}
