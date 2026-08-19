# Releasing

This repository uses a two-stage release process:

1. `release-please` creates and updates a release PR from Conventional Commits.
2. A maintainer publishes provider artifacts through the protected `publish` environment.

The release PR updates `CHANGELOG.md` and `.release-please-manifest.json`. While this repository is private and prelaunch, the release-please workflow intentionally uses `skip-github-release: true`; it does not create tags or GitHub Releases automatically.

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

For public Terraform Registry publishing, the repository should be public and named `terraform-provider-openai`. Until then, test the provider through a local development override, a filesystem mirror, or an HCP Terraform private registry.

## Publishing Artifacts

Provider consumers do not install directly from the Go source repository in normal Terraform usage. Terraform resolves provider versions through a provider registry and downloads signed release artifacts.

Terraform provider registries require signed provider checksums. The public Terraform Registry and HCP Terraform private registry both rely on GPG signatures for provider packages.

The release workflow reads signing credentials from the protected GitHub Actions environment named `publish`, not from repository-level secrets. Configure that environment with required reviewers or other protection rules before publishing.

The `publish` environment must define these environment secrets:

- `GPG_PRIVATE_KEY`: ASCII-armored private key used to sign release checksums.
- `PASSPHRASE`: passphrase for the private key.

To publish a provider version:

1. Merge the release-please PR.
2. Push a semver tag:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

The `Release` workflow waits for the `publish` environment checks, imports the GPG key from environment secrets, and runs GoReleaser. GoReleaser builds OS/architecture zip files, generates an SPDX JSON software bill of materials (SBOM) for each zip, includes the SBOMs in the signed checksum file, includes `terraform-registry-manifest.json`, and creates the GitHub Release.

CI builds a snapshot release and verifies that every provider zip has a non-empty SBOM listed in the checksum file. The tag-triggered `Release` workflow runs the same `Release SBOM` verification before its publish job, so a release cannot be published if that check fails. Both the snapshot and publish jobs first verify their own checked-out provider dependencies before building artifacts.

Once the public Terraform Registry is connected to the public repository, finalized GitHub Releases are ingested by the Registry.

## Release Security

The `publish` environment approval is the security boundary for release signing. Before approving a release job, reviewers should verify that the tag points at the intended reviewed commit and that the release workflow and `.goreleaser.yml` at that commit are expected.

Each release job first installs pinned GoReleaser and Syft release archives and authenticates each archive against a SHA-256 digest committed in `.github/actions/setup-release-tools/install.sh`. Tool installation fails closed if an archive cannot be downloaded, does not match its reviewed digest, or does not contain the expected executable; no mutable upstream installers or unauthenticated tool caches are used.

After all release tools are installed, the job creates new, empty Go module and build caches instead of trusting source archives, cache metadata, or compiled objects restored by `actions/setup-go`. It then downloads dependencies into the clean module cache using the committed checksums, runs `go mod verify`, and fails if either committed provider lockfile (`go.mod` or `go.sum`) changed. This final verification completes before the publish job accesses the GPG private key or passphrase. The already-authenticated local GoReleaser and Syft executables build with `-mod=readonly`, `GONOPROXY=none`, and `GOPROXY=off`, so only the freshly verified dependency cache and fresh compiled objects are used after signing credentials are imported.

GoReleaser does not automatically pass Go environment variables to SBOM subprocesses, and its default Syft arguments enable remote enrichment. Its reviewed `.goreleaser.yml` SBOM entry therefore invokes `.github/actions/verify-release-sboms/generate-sbom.sh`, passing both verified cache paths and the offline Go environment. That script invokes only the authenticated Syft executable with the explicitly committed `.github/actions/verify-release-sboms/syft.yaml` policy and a `file:` source. Ambient runner configuration cannot change catalogers, source identity, source transport, or enrichment settings. License enrichment reads only the verified module cache; remote enrichment, vendor cache access, executable package discovery, update checks, and persistent Syft caching are disabled. Syft's own module proxy is restricted to the verified local file cache because Syft interprets `GOPROXY=off` as direct network access.

Before GoReleaser can checksum or sign any SBOM, the same script verifies its SPDX source name, archive SHA-256 digest, source supplier, and complete module name/version inventory against the actual provider binary embedded in that archive. The OpenAI Go SDK must also retain its authenticated Apache-2.0 license. This check runs on the actual publishing runner and fails closed before signing; a successful snapshot on a different runner is never treated as proof that the published SBOM is safe. The release snapshot gate runs the same pipeline against an attacker-controlled home-directory configuration, poisoned module cache, module proxy, registry source selection, removed Go cataloger, and forged source identity.

When changing provider dependencies, run `go mod tidy` during normal development, commit the resulting `go.mod` and `go.sum` together, and verify them locally with `go mod download`, `go mod verify`, and `git diff --exit-code -- go.mod go.sum`. When upgrading GoReleaser or Syft, update its pinned version and every supported platform's reviewed SHA-256 digest in `.github/actions/setup-release-tools/install.sh`, review the committed Syft policy against the new version, then run the authentication regressions and adversarial full-release SBOM snapshot. Never add dependency-mutating hooks to `.goreleaser.yml`, allow ambient Syft configuration or source discovery, skip per-artifact verification before signing, enable remote SBOM enrichment, install tools after the final verification, restore a compiled build cache after verification, or import signing credentials before this boundary.

Recommended repository settings:

- Protect tags matching `v*`.
- Restrict who can create matching release tags.
- Require reviewers on the `publish` environment.
- Store `GPG_PRIVATE_KEY` and `PASSPHRASE` only as `publish` environment secrets.
