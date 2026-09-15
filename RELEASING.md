# Releasing

This repository uses a two-stage release process:

1. `release-please` creates and updates a release PR from Conventional Commits.
2. A maintainer publishes provider artifacts through the protected `publish` environment.

The release PR updates `CHANGELOG.md` and `.release-please-manifest.json`. The
release-please workflow intentionally uses `skip-github-release: true`; it does
not create tags or GitHub Releases automatically. Artifact publication remains
an explicit, protected tag-triggered step.

## Provider Name

The durable provider address should be:

```hcl
terraform {
  required_providers {
    openai = {
      source  = "openai/openai"
      version = "~> 0.1"
    }
  }
}
```

This repository is public, has the `terraform-provider-openai` name required
for public Terraform Registry publishing, and publishes under the durable
`openai/openai` address. Test unreleased provider changes through a local
development override, a filesystem mirror, or an HCP Terraform private
registry.

## Publishing Artifacts

Provider consumers do not install directly from the Go source repository in normal Terraform usage. Terraform resolves provider versions through a provider registry and downloads signed release artifacts.

Terraform provider registries require signed provider checksums. The public Terraform Registry and HCP Terraform private registry both rely on GPG signatures for provider packages.

The release workflow reads signing credentials from the protected GitHub Actions environment named `publish`, not from repository-level secrets. Configure that environment with required reviewers or other protection rules before publishing.

The `publish` environment must define these environment secrets:

- `GPG_PRIVATE_KEY`: ASCII-armored private key used to sign release checksums.
- `PASSPHRASE`: passphrase for the private key.

To publish a provider version:

1. Verify the release PR's title, changelog, and manifest agree on a version
   that has not already been published, then merge the reviewed release-please PR.
2. Tag that exact merge commit with its manifest version, rather than tagging
   whatever commit happens to be checked out. For example, for a release PR
   recording version `1.1.1`, substitute its verified full merge SHA below:

   ```sh
   git tag v1.1.1 <release-pr-merge-sha>
   git push origin v1.1.1
   ```

Before building release artifacts, the workflow requires the tag to match the
committed manifest and changelog and to point at the matching merged release PR
in public `main`. A mismatch fails before signing credentials are accessed.

Release-please uses `autorelease: pending` to discover and update release PRs;
keep labeling enabled. After the protected publication job succeeds, a separate
job verifies that the release is public and replaces that PR's pending label
with `autorelease: tagged`, preserving unrelated labels. This job has no signing
credentials. Failed publication retains pending status and blocks another
release PR. If label completion fails after successful publication, rerun only
the failed job; completion is idempotent. Do not rerun the successful publication
job or move an existing version tag. The next push to `main` lets release-please
prepare subsequent changes once pending status has cleared.

If an already-published version is missing from the manifest or changelog,
reconcile those files in a reviewed PR using the existing tag's actual commit
range and publication date. Never move tags or replace published artifacts to
repair bookkeeping, and never include later changes in a historical entry.

The `Release` workflow waits for the `publish` environment checks, imports the GPG key from environment secrets, and runs GoReleaser. GoReleaser builds OS/architecture zip files, generates an SPDX JSON software bill of materials (SBOM) for each zip, includes `terraform-registry-manifest.json`, and creates a draft GitHub Release. GoReleaser limits its checksum inventory to provider archives and the Registry manifest, including when it refreshes checksums after signing. The trusted release verifier snapshots each adjacent SBOM, validates the complete archive/SBOM inventory, then signs two inventories: `terraform-provider-openai_VERSION_SHA256SUMS` contains only provider ZIPs and the Registry manifest, while `terraform-provider-openai_VERSION_sbom_checksums.txt` contains the SBOMs. Both have detached `.sig` signatures from the same release key. SBOM entries must not appear in the Registry checksum file: Registry ingestion rejects those entries as missing from its request even when the SBOM files exist on GitHub. The approved publishing job generates GitHub OIDC-backed build-provenance attestations for both signed inventories, covering every provider archive, SBOM, and Registry manifest. It verifies every provider archive's attestation against this repository, release workflow, version tag, and source commit before publishing the draft. A missing, invalid, or mismatched attestation blocks publication.

CI builds a snapshot release and verifies every provider ZIP checksum and its adjacent SBOM against the exact archive contents and dependency inventory. Offline signing and publication regressions exercise the split inventories; the publication gate downloads and verifies both signed checksum files and all referenced artifacts before making the draft public. The tag-triggered `Release` workflow runs the same `Release SBOM` verification before its publish job, so a release cannot be published if that check fails. Both the snapshot and publish jobs first verify their own checked-out provider dependencies before building artifacts.

For a packaging correction, publish a new version through this process; never replace an existing release’s checksum files, signatures, or other artifacts. After publication, verify that the Registry lists the new version and advertises the expected archive digests and signing key. A successful GitHub release alone does not establish Registry ingestion.

The public Terraform Registry is connected to this repository and ingests
finalized GitHub Releases.

## Release Security

The `publish` environment approval is the security boundary for release signing and OIDC-backed provenance. Before approving a release job, reviewers should verify that the tag points at the intended reviewed commit and that the release workflow and `.goreleaser.yml` at that commit are expected. Only this approved job receives `id-token: write` and `attestations: write`; the preceding snapshot-verification job remains read-only. Terraform Registry still requires GPG-signed provider checksums, so the GitHub attestation supplements rather than replaces the GPG signature.

Each release job first installs pinned GoReleaser and Syft release archives and authenticates each archive against a SHA-256 digest committed in `.github/actions/setup-release-tools/install.sh`. Tool installation fails closed if an archive cannot be downloaded, does not match its reviewed digest, or does not contain the expected executable; no mutable upstream installers or unauthenticated tool caches are used.

After all release tools are installed, each job first sources the reviewed `.github/actions/verify-release-dependencies/trusted-release-environment.sh` policy before running Go or Git, or accessing signing credentials. The policy disables ambient Go configuration with `GOENV=off`, clears `GOFLAGS` and executable cache/compiler overrides, disables workspace discovery with `GOWORK=off`, selects only the authenticated local toolchain with `GOTOOLCHAIN=local`, disables CGO, and derives `GOROOT` from the trusted setup-go executable. It also ignores inherited global/system Git configuration and command-line configuration overrides, explicitly disables Git hooks, `core.fsmonitor`, external diffs, credential helpers, and interactive executables, and pins Git's executable root. These values are exported for immediate verification and persisted through `GITHUB_ENV` for every later release command; the SBOM subprocess also receives the same critical Go execution settings. Inherited executable Git hooks, `-toolexec`, source overlays, external cache programs, workspaces, and replacement toolchains therefore cannot escape the signing boundary.

The job then creates new, empty Go module and build caches instead of trusting source archives, cache metadata, or compiled objects restored by `actions/setup-go`. It downloads dependencies into the clean module cache using the committed checksums, runs `go mod verify`, and fails if either committed provider lockfile (`go.mod` or `go.sum`) changed. This final verification completes before the publish job accesses the GPG private key or passphrase. The already-authenticated local GoReleaser and Syft executables build with `-mod=readonly`, `GONOPROXY=none`, and `GOPROXY=off`, so only the freshly verified dependency cache and fresh compiled objects are used after signing credentials are imported.

GoReleaser does not automatically pass Go environment variables to SBOM subprocesses, and its default Syft arguments enable remote enrichment. Its reviewed `.goreleaser.yml` SBOM entry therefore invokes `.github/actions/verify-release-sboms/generate-sbom.sh`, passing both verified cache paths and the offline Go environment. That script invokes only the authenticated Syft executable with the explicitly committed `.github/actions/verify-release-sboms/syft.yaml` policy and a `file:` source. Ambient runner configuration cannot change catalogers, source identity, source transport, or enrichment settings. License enrichment reads only the verified module cache; remote enrichment, vendor cache access, executable package discovery, update checks, and persistent Syft caching are disabled. Syft's own module proxy is restricted to the verified local file cache because Syft interprets `GOPROXY=off` as direct network access.

Before GoReleaser can checksum or sign any SBOM, the same script verifies its SPDX source name, archive SHA-256 digest, source supplier, and complete module name/version inventory against the actual provider binary embedded in that archive. The OpenAI Go SDK must also retain its authenticated Apache-2.0 license. This check runs on the actual publishing runner and fails closed before signing; a successful snapshot on a different runner is never treated as proof that the published SBOM is safe. The release snapshot gate runs the same pipeline against an attacker-controlled home-directory configuration, poisoned module cache, module proxy, registry source selection, removed Go cataloger, and forged source identity.

When changing provider dependencies, run `go mod tidy` during normal development, commit the resulting `go.mod` and `go.sum` together, and verify them locally with `go mod download`, `go mod verify`, and `git diff --exit-code -- go.mod go.sum`. When upgrading Go, GoReleaser, or Syft, update the applicable pinned version and every supported platform's reviewed SHA-256 digest, review both the trusted release execution and Syft policies against the new versions, then run the authentication, hostile-Go/Git-environment, and adversarial full-release SBOM regressions. Never add dependency-mutating hooks to `.goreleaser.yml`, allow inherited Go execution flags, executable Git configuration, or ambient Syft configuration, skip per-artifact verification before signing, enable remote SBOM enrichment, install tools after the final verification, restore a compiled build cache after verification, or import signing credentials before this boundary.

Recommended repository settings:

- Protect tags matching `v*`.
- Restrict who can create matching release tags.
- Require reviewers on the `publish` environment.
- Store `GPG_PRIVATE_KEY` and `PASSPHRASE` only as `publish` environment secrets.
