# Releasing

Releases are driven by Release Please. Merging an approved release PR is the
human approval step:

1. Release Please maintains the changelog and version manifest in a release PR.
2. After merge, the SDK GitHub App creates the version tag at that release
   commit and a draft GitHub Release with the release notes.
3. The tag starts `Release`, which verifies, signs, attests, and publishes the
   provider artifacts through the `publish` environment.

`draft: true` keeps incomplete artifacts out of the Registry.
`force-tag-creation: true` is also required: GitHub does not normally create a
Git tag for a draft release until publication. The App token allows the tag
creation to trigger the separate workflow; the default `GITHUB_TOKEN` would not.

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

The release workflow reads signing credentials from the protected GitHub Actions environment named `publish`, not from repository-level secrets. Restrict that environment to release tags as described below; release-PR code-owner review provides approval without a second manual deployment approval.

The `publish` environment must define these environment secrets:

- `GPG_PRIVATE_KEY`: ASCII-armored private key used to sign release checksums.
- `PASSPHRASE`: passphrase for the private key.

To publish a provider version:

1. Verify the release PR's title, changelog, and manifest agree on a version
   that has not already been published, then merge the reviewed release-please PR.
2. Confirm the Release Please run creates the tag and draft automatically, then
   follow the tag-triggered `Release` run through publication and Registry
   ingestion. No manual tag push is part of the normal process.

Before building release artifacts, the workflow requires the tag to match the
committed manifest and changelog and to point at the matching merged release PR
in public `main`. A mismatch fails before signing credentials are accessed.
After snapshot verification, the publication job requires the Release Please
draft at that same commit before using signing credentials. Draft lookup uses
the publisher's existing contents-write token; the snapshot job stays read-only.

Release Please owns its `autorelease: pending` → `autorelease: tagged` label
transition when it creates the tag and draft. Tagged does not mean published.
After verified artifact publication, a separate job adds `autorelease: published`
without changing Release Please's labels or unrelated labels. Failed publication
leaves the release as a draft and does not receive the published label.

For a transient failure before artifact upload, rerun only the failed jobs of
the original release run. If Release Please created the tag but failed to create
the draft, rerun its failed job first, then rerun the failed Release jobs; do not
move the tag or fabricate an empty draft. After any artifacts have uploaded, a
retry can fail on duplicate assets: first verify the release is still an unpublished draft,
then deliberately remove its incomplete asset set before retrying. Preserve
the draft's release notes and the immutable tag; never remove or replace assets
of a published release. A repository code/configuration fix requires a new
reviewed version because retrying an old run still uses its tagged source.

If label completion fails after successful publication, rerun only that job;
completion is idempotent. Do not rerun a successful publication job. Subsequent
pushes can let Release Please prepare another version; check the prior release's
publication status before merging another release PR.

If an already-published version is missing from the manifest or changelog,
reconcile those files in a reviewed PR using the existing tag's actual commit
range and publication date. Never move tags or replace published artifacts to
repair bookkeeping, and never include later changes in a historical entry.

The `Release` workflow waits for the `publish` environment checks, imports the GPG key from environment secrets, and runs GoReleaser. GoReleaser builds OS/architecture zip files, generates an SPDX JSON software bill of materials (SBOM) for each zip, includes `terraform-registry-manifest.json`, and uploads them to Release Please's existing draft, preserving its release notes. GoReleaser limits its checksum inventory to provider archives and the Registry manifest, including when it refreshes checksums after signing. The trusted release verifier snapshots each adjacent SBOM, validates the complete archive/SBOM inventory, then signs two inventories: `terraform-provider-openai_VERSION_SHA256SUMS` contains only provider ZIPs and the Registry manifest, while `terraform-provider-openai_VERSION_sbom_checksums.txt` contains the SBOMs. Both have detached `.sig` signatures from the same release key. SBOM entries must not appear in the Registry checksum file: Registry ingestion rejects those entries as missing from its request even when the SBOM files exist on GitHub. The publishing job generates GitHub OIDC-backed build-provenance attestations for both signed inventories, covering every provider archive, SBOM, and Registry manifest. It verifies every provider archive's attestation against this repository, release workflow, version tag, and source commit before publishing the draft. A missing, invalid, or mismatched attestation blocks publication.

CI builds a snapshot release and verifies every provider ZIP checksum and its adjacent SBOM against the exact archive contents and dependency inventory. Offline signing and publication regressions exercise the split inventories; the publication gate downloads and verifies both signed checksum files and all referenced artifacts before making the draft public. The tag-triggered `Release` workflow runs the same `Release SBOM` verification before its publish job, so a release cannot be published if that check fails. Both the snapshot and publish jobs first verify their own checked-out provider dependencies before building artifacts.

For a packaging correction, publish a new version through this process; never replace an existing release’s checksum files, signatures, or other artifacts. After publication, verify that the Registry lists the new version and advertises the expected archive digests and signing key. A successful GitHub release alone does not establish Registry ingestion.

The public Terraform Registry is connected to this repository and ingests
finalized GitHub Releases.

## Release Security

Required code-owner review and checks on the release PR authorize publication.
The tag must point to that exact merged release commit, and the release workflow
and `.goreleaser.yml` are covered by `@openai/sdks-team` ownership. The
`publish` environment isolates GPG secrets and is restricted to `v*` tags; it
does not require another manual approval. Only its publishing job receives
`id-token: write` and `attestations: write`; the preceding snapshot-verification
job remains read-only. Terraform Registry still requires GPG-signed provider
checksums, so the GitHub attestation supplements rather than replaces the GPG
signature.

Each release job first installs pinned GoReleaser and Syft release archives and authenticates each archive against a SHA-256 digest committed in `.github/actions/setup-release-tools/install.sh`. Tool installation fails closed if an archive cannot be downloaded, does not match its reviewed digest, or does not contain the expected executable; no mutable upstream installers or unauthenticated tool caches are used.

After all release tools are installed, each job first sources the reviewed `.github/actions/verify-release-dependencies/trusted-release-environment.sh` policy before running Go or Git, or accessing signing credentials. The policy disables ambient Go configuration with `GOENV=off`, clears `GOFLAGS` and executable cache/compiler overrides, disables workspace discovery with `GOWORK=off`, selects only the authenticated local toolchain with `GOTOOLCHAIN=local`, disables CGO, and derives `GOROOT` from the trusted setup-go executable. It also ignores inherited global/system Git configuration and command-line configuration overrides, explicitly disables Git hooks, `core.fsmonitor`, external diffs, credential helpers, and interactive executables, and pins Git's executable root. These values are exported for immediate verification and persisted through `GITHUB_ENV` for every later release command; the SBOM subprocess also receives the same critical Go execution settings. Inherited executable Git hooks, `-toolexec`, source overlays, external cache programs, workspaces, and replacement toolchains therefore cannot escape the signing boundary.

The job then creates new, empty Go module and build caches instead of trusting source archives, cache metadata, or compiled objects restored by `actions/setup-go`. It downloads dependencies into the clean module cache using the committed checksums, runs `go mod verify`, and fails if either committed provider lockfile (`go.mod` or `go.sum`) changed. This final verification completes before the publish job accesses the GPG private key or passphrase. The already-authenticated local GoReleaser and Syft executables build with `-mod=readonly`, `GONOPROXY=none`, and `GOPROXY=off`, so only the freshly verified dependency cache and fresh compiled objects are used after signing credentials are imported.

GoReleaser does not automatically pass Go environment variables to SBOM subprocesses, and its default Syft arguments enable remote enrichment. Its reviewed `.goreleaser.yml` SBOM entry therefore invokes `.github/actions/verify-release-sboms/generate-sbom.sh`, passing both verified cache paths and the offline Go environment. That script invokes only the authenticated Syft executable with the explicitly committed `.github/actions/verify-release-sboms/syft.yaml` policy and a `file:` source. Ambient runner configuration cannot change catalogers, source identity, source transport, or enrichment settings. License enrichment reads only the verified module cache; remote enrichment, vendor cache access, executable package discovery, update checks, and persistent Syft caching are disabled. Syft's own module proxy is restricted to the verified local file cache because Syft interprets `GOPROXY=off` as direct network access.

Before GoReleaser can checksum or sign any SBOM, the same script verifies its SPDX source name, archive SHA-256 digest, source supplier, and complete module name/version inventory against the actual provider binary embedded in that archive. The OpenAI Go SDK must also retain its authenticated Apache-2.0 license. This check runs on the actual publishing runner and fails closed before signing; a successful snapshot on a different runner is never treated as proof that the published SBOM is safe. The release snapshot gate runs the same pipeline against an attacker-controlled home-directory configuration, poisoned module cache, module proxy, registry source selection, removed Go cataloger, and forged source identity.

When changing provider dependencies, run `go mod tidy` during normal development, commit the resulting `go.mod` and `go.sum` together, and verify them locally with `go mod download`, `go mod verify`, and `git diff --exit-code -- go.mod go.sum`. When upgrading Go, GoReleaser, or Syft, update the applicable pinned version and every supported platform's reviewed SHA-256 digest, review both the trusted release execution and Syft policies against the new versions, then run the authentication, hostile-Go/Git-environment, and adversarial full-release SBOM regressions. Never add dependency-mutating hooks to `.goreleaser.yml`, allow inherited Go execution flags, executable Git configuration, or ambient Syft configuration, skip per-artifact verification before signing, enable remote SBOM enrichment, install tools after the final verification, restore a compiled build cache after verification, or import signing credentials before this boundary.

## Repository setup

The workflow configuration and live GitHub settings must agree. Before enabling
automatic releases, configure and read back these settings:

- Require `@openai/sdks-team` code-owner review and CI before release PRs merge.
- Keep `release` restricted to `main`, with the SDK App credential scoped to that
  environment and contents/pull-request write permissions.
- Permit the SDK GitHub App to create `v*` tags. Keep tag update/deletion
  restrictions in a separate ruleset without an App bypass, so creation
  permission does not allow changing existing releases. Preserve existing
  maintainer access.
- Restrict `publish` to tags matching `v*`, remove its additional required-reviewer
  rule, and preserve its environment secrets. The release-PR review and exact
  merge-commit verification supply the approval boundary.
- Store `GPG_PRIVATE_KEY` and `PASSPHRASE` only as `publish` environment secrets.

These are repository-admin settings, not changes a workflow YAML file can apply.
The first release after setup must verify automatic tag/draft creation, the
triggered Release run, both signed inventories, provenance, and Registry
installation. Never publish an empty release to test this path.
