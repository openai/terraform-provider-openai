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

The `Release` workflow waits for the `publish` environment checks, imports the GPG key from environment secrets, and runs GoReleaser. GoReleaser builds OS/architecture zip files, generates checksums, signs the checksum file, includes `terraform-registry-manifest.json`, and creates the GitHub Release.

Once the public Terraform Registry is connected to the public repository, finalized GitHub Releases are ingested by the Registry.

## Release Security

The `publish` environment approval is the security boundary for release signing. Before approving a release job, reviewers should verify that the tag points at the intended reviewed commit and that the release workflow and `.goreleaser.yml` at that commit are expected.

Recommended repository settings:

- Protect tags matching `v*`.
- Restrict who can create matching release tags.
- Require reviewers on the `publish` environment.
- Store `GPG_PRIVATE_KEY` and `PASSPHRASE` only as `publish` environment secrets.
