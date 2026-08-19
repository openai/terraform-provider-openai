# Agent instructions

Read [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and
[RELEASING.md](RELEASING.md) before changing this repository.

## Generated code and repository ownership

- The provider manages OpenAI Administration API resources with privileged
  organization credentials. Treat authentication, authorization, Terraform
  state, and published provider artifacts as security-sensitive boundaries.
- Check `.terraform-generator-manifest.json` before editing provider sources,
  generated tests, Terraform examples, or `docs/`. Files listed in the manifest
  and files marked `Code generated ... DO NOT EDIT` are generator-owned; change
  their authoritative upstream source when available instead of introducing an
  uncoordinated manual patch. `make generate` regenerates provider documentation,
  not the upstream provider implementation.
- Preserve the existing `@openai/sdks-team` ownership and obtain appropriate
  security-focused review for changes affecting the boundaries below.

## Security requirements

- Never commit, print, or upload real admin or API keys, bearer tokens, GitHub
  App private keys, GPG keys, signing passphrases, `.env` files, customer data,
  Terraform state, saved plans, or sensitive `.tfvars` files. Read the provider's
  organization-admin credential from `OPENAI_ADMIN_KEY`; do not substitute
  `OPENAI_API_KEY` or place a live key in Terraform configuration or examples.
- Use clearly fake credentials, synthetic organization/project/user data,
  `t.Setenv`, and `httptest` or local mocks in ordinary tests and fixtures.
  Acceptance tests require an explicit `TF_ACC=1`, `OPENAI_ADMIN_KEY`, and
  applicable `OPENAI_TF_ACC_*` values; they can mutate real organizations, users,
  roles, projects, certificates, retention settings, and spend controls. Never
  enable them automatically or run them without an authorized isolated account.
- Redact `Authorization` and other credential-bearing headers, cookies, signed
  URLs, private key material, customer identifiers, email addresses, and
  sensitive request or response bodies from diagnostics, fixtures, Terraform
  logs, crash reports, traces, and CI artifacts. Treat `TF_LOG`,
  `TF_LOG_PROVIDER_OPENAI_CLIENT`, saved plans, and `*.tfstate` as sensitive;
  Terraform's `Sensitive: true` or `sensitive = true` hides normal display but
  does not guarantee that a value is absent from state or plan files.
- Review direct and transitive dependencies, provenance, versions, and `replace`
  directives in both `go.mod`/`go.sum` and `tools/go.mod`/`tools/go.sum`.
  Preserve Go module checksum verification; inspect generator/install scripts,
  GoReleaser hooks, Syft downloads, Terraform binaries, provider sources, and
  any `.terraform.lock.hcl` before accepting or executing supply-chain changes.
- Pin third-party GitHub Actions to reviewed full commit SHAs. Keep workflow
  permissions and GitHub App tokens least-privileged; preserve
  `permissions: {}` where present, isolate write-capable tokens and secrets from
  untrusted pull-request code, and keep release-only access in the existing
  protected `release` and `publish` environments.
- Protect `OPENAI_SDKS_APP_PRIVATE_KEY`, `GPG_PRIVATE_KEY`, `PASSPHRASE`, and
  publishing tokens. Preserve the reviewed tag-triggered GoReleaser workflow,
  protected signing credentials, OS/architecture provider archives,
  per-archive SPDX SBOMs, signed SHA-256 checksums covering those SBOMs, the
  Terraform Registry manifest, and mandatory pre-publication SBOM verification.
- Obtain security-focused `@openai/sdks-team` review and add targeted offline
  regression tests for changes to admin authentication; `base_url`, redirects,
  proxies, TLS, or credential forwarding; path/import-ID validation;
  organization/project authorization, role grants, or service accounts;
  Terraform schema sensitivity, state, plans, or diagnostics; certificates,
  data retention, spend controls, retries, caching, or pagination; and
  dependency, generation, CI, signing, or release behavior. Preserve
  `create_service_account_only=true`: service-account resources must not create
  API keys, grant roles implicitly, or persist credentials in Terraform state.
- Report suspected vulnerabilities in the provider or its release artifacts
  privately to `disclosure@openai.com` as described in
  [SECURITY.md](SECURITY.md), including the affected version/artifact and
  sanitized reproduction details.
  Do not report security vulnerabilities through public GitHub issues, pull
  requests, or discussions. Do not include live credentials, API keys, customer
  data, or unredacted sensitive logs.
