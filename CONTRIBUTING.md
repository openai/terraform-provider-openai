# Contributing to the OpenAI Terraform provider

Thank you for helping improve the `openai/openai` Terraform provider. Read
[README.md](README.md), [SECURITY.md](SECURITY.md), and
[RELEASING.md](RELEASING.md) before changing provider behavior or release
artifacts. The repository's [CODEOWNERS](.github/CODEOWNERS) identify the SDK
maintainers responsible for review.

## Where contributions land

Provider pull requests from maintainers and collaborators land in this public
repository. Its `main` branch is the reviewed source for provider changes and
releases. Repository policy currently limits pull-request creation to
collaborators; external contributors should open a non-security issue describing
the proposed change so a maintainer can coordinate it. Report vulnerabilities
privately as described in [SECURITY.md](SECURITY.md), not through an issue.

An internal repository is intended to receive a one-way operational mirror of
public `main`; it is not a separate contribution surface, and pull requests
opened there are not imported back here. A checked-in mirror configuration does
not by itself prove that the mirror is deployed or current.

Repository mirroring and provider generation are independent. Mirroring copies
an already-reviewed public Git commit; it does not regenerate provider code.

## Development and file ownership

Use the Go version declared in `go.mod` and a supported Terraform CLI. Review
dependency origins before downloading modules or running generator, install, or
release tooling.

Before editing a file, consult `.terraform-generator-manifest.json` and any
`Code generated ... DO NOT EDIT` header:

- Files listed in the manifest are generator-owned. This includes most provider
  implementation, resource tests, Terraform examples, and generated
  documentation. Do not hand-edit them as the durable fix.
- `.terraform-generator-manifest.json` is itself generator-maintained and is
  rewritten by incremental generation; do not edit its ownership set or hashes
  by hand even though it cannot list itself.
- Other files absent from the manifest and without a generated header are
  maintained in this repository. Changes to those files can be proposed
  directly here, subject to `CODEOWNERS` review.

The provider uses a purpose-built Terraform generator in the OpenAI monorepo,
not Castiron. For a generator-owned change, external contributors should open a
non-security issue here and coordinate with `@openai/sdks-team`; an OpenAI
maintainer will make the authoritative generator or input change and regenerate
the provider.

For maintainers, the authoritative generator lives under
[`project/terraform-generator`](https://github.com/openai/openai/tree/master/project/terraform-generator).
The update flow is:

1. Change and test the generator, provider config, templates, handwritten API
   reference sources, or owning service declarations for generated public
   OpenAPI JSON in the monorepo.
2. If needed, run `prepare_openapi_input.py` to materialize the local untracked
   OpenAPI bundle. This preparation step does not generate or publish a provider.
3. Run the generator incrementally against a linked checkout of this repository.
4. Review the generated files and `.terraform-generator-manifest.json`, run the
   configured quality checks and provider tests, and submit the provider diff as
   a pull request here.
5. Merge the reviewed public provider change. Repository mirroring, when
   deployed and verified by its operators, subsequently makes the exact public
   `main` commit available in the internal mirror.

`make generate` only refreshes provider documentation from the current provider
and examples. It does not run the upstream generator or regenerate the provider
implementation.

## Security requirements

### Credentials, Terraform state, and safe examples

- Never commit real OpenAI Admin API keys, access tokens, GitHub App private
  keys, GPG private keys, signing passphrases, `.env` files, customer data,
  Terraform state, saved plans, or sensitive `.tfvars` files. Supply the
  provider's administrator credential through `OPENAI_ADMIN_KEY`; never use a
  live key in `.tf` files, examples, fixtures, command arguments, or shell
  history. A standard `OPENAI_API_KEY` is not a substitute for the Admin API key.
- Use clearly fake credentials, synthetic organization/project/user data, and
  local `httptest` servers or mocks in unit tests, examples, and recorded
  responses. Keep ordinary unit tests offline and free of real credentials.
- Preserve `Sensitive: true` on credential-bearing provider attributes and
  `sensitive = true` on Terraform variables where appropriate. Treat state,
  saved plans, `.terraform/`, and crash logs as confidential: marking a value
  sensitive limits normal display but does not prevent sensitive data from
  being retained in Terraform state or plan files.
- Redact `Authorization`, cookies, signed URLs, secret-bearing query strings,
  private key material, customer identifiers, emails, and sensitive request or
  response bodies from Terraform diagnostics, `TF_LOG`,
  `TF_LOG_PROVIDER_OPENAI_CLIENT`, fixtures, snapshots, and CI artifacts.

### Dependency and supply-chain review

- Review direct and transitive modules, their provenance, version changes, and
  new `replace` directives in `go.mod`, `go.sum`, `tools/go.mod`, and
  `tools/go.sum`. Preserve Go checksum verification and reject unexplained
  lockfile changes.
- Review `go generate`, Terraform/provider downloads, documentation-generator
  dependencies, GoReleaser hooks, Syft versions, and `.terraform.lock.hcl`
  changes when present. Verify provider registry sources and published release
  checksums before trusting downloaded artifacts.
- Pin third-party GitHub Actions to reviewed full immutable commit SHAs. Grant
  CI jobs and GitHub App tokens only their required permissions; never expose
  secrets or write-capable tokens to untrusted pull-request code.

### Release signing and published artifacts

The provider is distributed as signed release artifacts, not as a Go package
that Terraform users install directly. Preserve the separate release-please and
tag-triggered publication stages described in [RELEASING.md](RELEASING.md),
including the protected `release` and `publish` environments.

Keep `OPENAI_SDKS_APP_PRIVATE_KEY`, `GPG_PRIVATE_KEY`, `PASSPHRASE`, and
publishing credentials in their approved protected environments. Never commit
signing material or expose it to shell history, logs, artifacts, untrusted
scripts, or pull requests. Preserve reviewed version tags, GoReleaser's
OS/architecture archives, a valid SPDX SBOM for every archive, signed SHA-256
checksums that include those SBOMs, the Terraform Registry manifest, and both
CI and pre-publication SBOM verification.

### Security-sensitive changes and testing

Obtain focused `@openai/sdks-team` review and add targeted offline regression
tests for changes to authentication or credential forwarding; `base_url`,
redirects, proxies, and TLS; path parameters or Terraform import identifiers;
organization/project isolation, role grants, users, or service accounts;
provider-schema sensitivity and state handling; request diagnostics, caching,
pagination, and mutation retries; certificates, retention, and spend limits;
dependency installation, code generation, CI, release signing, or publication.

Preserve `create_service_account_only=true` for project service accounts.
Service-account resources must not create API keys, assign roles implicitly, or
store credentials in Terraform state. Manage role assignments separately and
keep API-key creation outside Terraform.

Run focused offline unit tests or the standard unit-test suite in a subshell
that removes acceptance-test opt-in, live credentials, organization/project
identifiers, and every acceptance fixture while preserving unrelated settings:

```sh
offline_go_test() (
  unset TF_ACC OPENAI_ADMIN_KEY OPENAI_API_KEY \
    OPENAI_ORG_ID OPENAI_ORGANIZATION OPENAI_ORGANIZATION_ID \
    OPENAI_PROJECT OPENAI_PROJECT_ID || exit 1

  fixture_names=$(env | awk -F= \
    '$1 ~ /^OPENAI_TF_ACC_[A-Za-z_][A-Za-z0-9_]*$/ { print $1 }') || exit 1
  for name in $fixture_names; do
    unset "$name" || exit 1
  done

  go test "$@"
)

offline_go_test ./internal/provider/openaiapi ./internal/provider
offline_go_test ./...
```

Check Terraform example formatting when relevant:

```sh
terraform fmt -check -recursive examples
```

Acceptance tests are explicitly opt-in and require `TF_ACC=1`,
`OPENAI_ADMIN_KEY`, and applicable `OPENAI_TF_ACC_*` fixture variables. They can
create, modify, or delete real organization resources and may affect access,
retention, or spending. Run `make testacc` or its narrower targets only against
an authorized isolated organization with approved credentials; never enable
acceptance tests automatically in pull-request workflows.

## Reporting vulnerabilities

Report suspected vulnerabilities in the Terraform provider or its release
artifacts privately to `disclosure@openai.com` through
[SECURITY.md](SECURITY.md). Include the affected provider version or release
artifact, the security impact, and sanitized steps to reproduce the issue.

Do not report security vulnerabilities through public GitHub issues, pull
requests, or discussions. Do not include live credentials, API keys, customer
data, or unredacted sensitive logs.
