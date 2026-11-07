# Agent instructions

Before changes, read and follow [CONTRIBUTING.md](CONTRIBUTING.md),
[SECURITY.md](SECURITY.md), and [RELEASING.md](RELEASING.md).

## Generated code and repository ownership

- Follow [file ownership and upstream generation](CONTRIBUTING.md#development-and-file-ownership): check `.terraform-generator-manifest.json` and generated headers before editing provider sources, tests, examples, or docs. Fix generator-owned files upstream, not through uncoordinated manual patches. `make generate` refreshes documentation, not provider implementation.
- Preserve `@openai/sdks-team` ownership. Administration API resources use privileged organization credentials; beta project webhooks use project credentials. Authentication, authorization, Terraform state, and published artifacts are security boundaries.

## Security requirements

- Follow [credential, state, and example rules](CONTRIBUTING.md#credentials-terraform-state-and-safe-examples). Never commit, print, or upload real credentials/signing material, `.env`, customer data, state, saved plans, or sensitive `.tfvars`. Redact sensitive diagnostics, logs, crash reports, traces, and artifacts; Terraform sensitivity flags do not remove data from state/plans.
- Use `OPENAI_ADMIN_KEY` for organization administration and `OPENAI_API_KEY` for project webhooks; never substitute them or embed live keys in configuration/examples. Ordinary tests use fake credentials, synthetic data, `t.Setenv`, and `httptest`/local mocks.
- Follow [security-sensitive review and offline testing](CONTRIBUTING.md#security-sensitive-changes-and-testing), including focused SDK-team review and targeted offline regressions for every listed boundary. Acceptance tests require explicit `TF_ACC=1`, audience-specific credentials, applicable `OPENAI_TF_ACC_*` fixtures, and an authorized isolated account/project; never enable automatically. They mutate real resources.
- Preserve `create_service_account_only=true`: service-account resources must not create API keys, grant roles implicitly, or persist credentials in state.
- Follow [dependency/supply-chain review](CONTRIBUTING.md#dependency-and-supply-chain-review) for both provider and tools modules, scripts, downloads, and lockfiles. Preserve checksum verification, reviewed full-SHA Actions, least-privilege tokens, existing `permissions: {}`, and isolation from untrusted PR code.
- Preserve protected `release`/`publish` environments and the [signing/artifact requirements](CONTRIBUTING.md#release-signing-and-published-artifacts), including per-archive SPDX SBOMs, signed SHA-256 coverage, Registry manifest, and pre-publication verification. Follow RELEASING.md for the tag-triggered publication workflow.
- Report vulnerabilities privately to `disclosure@openai.com` per [SECURITY.md](SECURITY.md), with affected version/artifact and sanitized reproduction; never disclose through public issues, PRs, or discussions.
