# Contributing to the OpenAI Terraform provider

Thank you for helping improve the `openai/openai` Terraform provider. Read
[README.md](README.md), [SECURITY.md](SECURITY.md), and
[RELEASING.md](RELEASING.md) before changing provider behavior or release
artifacts. The repository's [CODEOWNERS](.github/CODEOWNERS) identify the SDK
maintainers responsible for review.

## Development and generated code

Use the Go version declared in `go.mod` and a supported Terraform CLI. Review
dependency origins before downloading modules or running generator, install, or
release tooling.

Most provider implementation, resource tests, Terraform examples, and generated
documentation are owned by the upstream generator. Consult
`.terraform-generator-manifest.json` and `Code generated ... DO NOT EDIT`
headers before editing. Prefer a change to the authoritative generator or
upstream source; do not hand-edit generated files without coordinating with the
maintainers. `make generate` refreshes provider documentation from the current
provider and examples; it does not regenerate the provider implementation.

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

Run focused offline unit tests or the standard unit-test suite with `TF_ACC`
explicitly removed, even when it was exported by an earlier acceptance run:

```sh
env -u TF_ACC go test ./internal/provider/openaiapi ./internal/provider
env -u TF_ACC go test ./...
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
