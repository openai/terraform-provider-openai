# Security Model

This document is the single canonical detailed threat model for the OpenAI
Terraform provider. Codex Security scans and human reviewers should use this
model for trust-boundary authority. [SECURITY.md](../../SECURITY.md) remains the
disclosure and reportability policy.

## 1. Overview

The provider is a Terraform protocol plugin that manages privileged OpenAI
Administration API resources, project-scoped webhook endpoints, and workspace
and tenant workload identity resources. Terraform
loads the provider, supplies configuration, plan, state, and import values, and
the provider translates those values into requests to the configured API
origin. Each resource selects its own credential audience: Platform Admin,
project, workspace admin, or tenant admin. Credentials never fall back across
audiences. The
provider bounds GET and paginated API responses, parses API responses,
validates modeled fields, and some data sources intentionally persist the
bounded raw GET response as `response_json` state. Mutation responses use the
SDK decoder under request-lifecycle timeout and retry controls, but do not
currently share those repository byte budgets. The provider identifies itself
to Terraform with the
durable address
`registry.terraform.io/openai/openai`, published from this public repository.
Consumers install releases through the Terraform Registry and test unreleased
changes through a local development override, filesystem mirror, or private
registry ([main.go](../../main.go#L16-L24),
[RELEASING.md](../../RELEASING.md#L8-L32)).

| Component | Responsibility | Evidence |
| --- | --- | --- |
| Provider configuration | Selects Platform Admin, project, workspace admin, and tenant admin credentials and their endpoints; creates isolated clients without cross-audience fallback. Workspace and tenant clients do not send Platform organization or project headers. | [internal/provider/provider.go](../../internal/provider/provider.go), [internal/provider/provider_security.go](../../internal/provider/provider_security.go) |
| API client | Expands fixed route templates, escapes path parameters, issues lifecycle-bounded requests, applies byte budgets to GET/paginated responses, parses responses, paginates, caches reads, and records redacted lifecycle telemetry. | [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L293-L325), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L337-L456), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L811-L861), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L870-L929), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L946-L1144) |
| Generated resources and data sources | Map Terraform schemas and CRUD/read operations to Platform, workspace, and tenant API endpoints, selecting the declared credential audience during configuration. | [internal/provider/provider.go](../../internal/provider/provider.go) |
| Service-account resource | Creates only a service account and deliberately does not create an API key or assign a role. | [internal/provider/resources/project_service_account/resource_project_service_account.go](../../internal/provider/resources/project_service_account/resource_project_service_account.go#L44-L46), [internal/provider/resources/project_service_account/resource_project_service_account.go](../../internal/provider/resources/project_service_account/resource_project_service_account.go#L121-L159) |
| Ordinary CI | Builds, tests, lints, formats examples, checks generated docs, and verifies release SBOMs with read-only repository permission. | [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15), [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L43-L162) |
| Release workflows | Create release PRs, automatically tag their merged commits with draft releases, then publish verified signed artifacts through scoped environments. | [.github/workflows/release-please.yml](../../.github/workflows/release-please.yml#L8-L32), [.github/workflows/release.yml](../../.github/workflows/release.yml#L12-L109) |

### Effective resources and capabilities

| Deployment or workflow | Resource or capability | Configuration and precedence | Safe effective value or location | Readers, writers, or recipients | Enforcing control | Evidence or unknowns |
| --- | --- | --- | --- | --- | --- | --- |
| Normal provider execution | Audience-specific credentials and request destination | Each explicit credential overrides its corresponding environment variable. `base_url`, `workspace_base_url`, and `tenant_base_url` select the respective endpoints. Ambient project, workspace, and tenant credentials are used only with their default origins. | Credential values are secret. Workspace requests default to `https://api.chatgpt.com/v1`; other audiences default to `https://api.openai.com/v1`. | The matching audience client and configured API origin. | All four credentials are schema-sensitive; resources select one audience without fallback. Custom origins require explicit project, workspace, or tenant credentials. URL validation rejects embedded credentials and non-loopback plaintext HTTP; redirects stay on the configured origin. A lower-trust config author must not control an endpoint paired with a separately injected Admin API key or explicit credential. | [internal/provider/provider.go](../../internal/provider/provider.go), [internal/provider/provider_security.go](../../internal/provider/provider_security.go) |
| Terraform state and plans | Organization/project resource state, webhook signing secrets, certificate-related fields, and raw `response_json` on data sources | Terraform configuration, remote API responses, and Terraform backend policy. | Caller-managed Terraform state and plan storage; no repository-owned backend is configured here. Webhook signing secrets are returned only on creation and remain in state; raw response JSON can contain API-returned fields beyond modeled attributes. Both must be treated as confidential. | Terraform operator and backend-authorized readers. | `Sensitive: true` limits normal display for sensitive schema fields but does not remove values from state or plans. Webhook reads and updates preserve the create-time secret, while import cannot recover it; raw `response_json` is intentionally persisted by data sources. | [internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go](../../internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go#L116-L126), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L1450-L1456), [internal/provider/resources/certificate/data_source_certificate.go](../../internal/provider/resources/certificate/data_source_certificate.go#L92-L98), [CONTRIBUTING.md](../../CONTRIBUTING.md#L85-L93); backend controls are external. |
| Ordinary pull-request CI | Execution of checked-in build, test, generation, and verification code | Workflow checked out at the proposed revision. | Ephemeral GitHub Actions runner. | PR code can execute in that runner; repository permission is read-only. | Workflow-level `contents: read`; no protected release credentials are declared in this workflow. | [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15), [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L43-L162) |
| Release-please on main | GitHub App token for release PRs, tags, and draft releases | Push to `main`, then protected `release` environment. | Token derived from `OPENAI_SDKS_APP_PRIVATE_KEY`; literal secret is never repository data. | release-please action receives contents and pull-request write authority. | Empty workflow default permissions; token created only in the `release` environment with explicit scopes. | [.github/workflows/release-please.yml](../../.github/workflows/release-please.yml#L3-L32) |
| Tag-triggered publication | GPG signing key, passphrase, OIDC attestations, release publication | Tag matching `v*`, successful SBOM job, then protected `publish` environment. | Protected environment secrets and runner-local verified artifacts. | Publish job, GoReleaser, release verifier, GitHub attestations. | Separate pre-publish SBOM job; publish job has scoped permissions; tools, dependencies, SBOMs, checksums, and attestations are verified. | [.github/workflows/release.yml](../../.github/workflows/release.yml#L3-L33), [.github/workflows/release.yml](../../.github/workflows/release.yml#L44-L109), [.github/actions/setup-release-tools/install.sh](../../.github/actions/setup-release-tools/install.sh#L28-L57), [.github/actions/verify-release-dependencies/action.yml](../../.github/actions/verify-release-dependencies/action.yml#L7-L22) |

## 2. Threat Model, Trust Boundaries, and Assumptions

### Protected assets and objectives

- The Platform Admin, project, workspace admin, and tenant admin credentials must reach only the operator-selected
  API origin and their declared resource audiences, with no cross-audience
  fallback. They must not appear in diagnostics, logs, fixtures, source, or
  ordinary display. The provider marks all four sensitive and creates a same-origin
  redirect policy
  ([internal/provider/provider.go](../../internal/provider/provider.go#L67-L84),
  [internal/provider/provider.go](../../internal/provider/provider.go#L113-L177),
  [internal/provider/provider_security.go](../../internal/provider/provider_security.go#L98-L117)).
- Organization and project resources—users, groups, roles, service accounts,
  certificates, retention settings, spend controls, permissions, and rate
  limits—must be mutated only through the caller's Admin API authority and the
  API's authorization checks ([internal/provider/provider.go](../../internal/provider/provider.go#L154-L225)).
- Webhook endpoints must be read and mutated only through project API authority
  with the applicable `api.webhooks.read` or `api.webhooks.write` scope. The
  provider must not substitute an Admin API credential when the project
  credential is absent
  ([internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go](../../internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go#L140-L166)).
- Terraform state and plans can contain confidential operational data; callers
  must protect their backend even where the schema suppresses normal display
  ([CONTRIBUTING.md](../../CONTRIBUTING.md#L85-L89)).
- Workspace WIF mutations require a workspace-owned provider and its exact
  workspace binding. A provider shared from a tenant may be read through a
  workspace data source but cannot be managed through a workspace resource.
  Tenant rules select exactly one workspace or API organization target; the
  API enforces that the target belongs to the credential's tenant. See the
  [WIF guide](../guides/workload-identity.md) and
  [workspace client adapter](../../internal/provider/openaiapi/client.go).
- A failed workspace binding step must leave enough Terraform state to recover
  or delete the provider already created. Refresh must remain read-only, and
  only documented not-found responses may remove a managed resource from state.
- A webhook signing secret must be captured only from the create response,
  marked sensitive, never sent in mutation bodies, and preserved rather than
  overwritten by later API responses. Import cannot recover the one-time value.
- Service-account creation must preserve
  `create_service_account_only=true`: no implicit API key, role grant, or
  credential persistence ([internal/provider/resources/project_service_account/resource_project_service_account.go](../../internal/provider/resources/project_service_account/resource_project_service_account.go#L44-L46),
  [internal/provider/resources/project_service_account/resource_project_service_account.go](../../internal/provider/resources/project_service_account/resource_project_service_account.go#L131-L159)).
- Published archives, SBOMs, checksums, signatures, and attestations must remain
  bound to the reviewed tag and protected publication workflow
  ([.github/workflows/release.yml](../../.github/workflows/release.yml#L88-L109),
  [.goreleaser.yml](../../.goreleaser.yml#L36-L86)).

### Actors and starting capabilities

- A Terraform operator holding an API key or workspace admin personal access token can
  intentionally configure the provider and request operations authorized for
  that credential audience. That authority is not a provider bypass.
- A Terraform configuration, state, or import author can supply attribute
  values, identifiers, and a configured HTTPS endpoint, but cannot change fixed
  checked-in route templates or grant themselves API authorization. If that
  author is less trusted than the runtime that independently injects
  `OPENAI_ADMIN_KEY` or supplies an explicit credential attribute, forwarding
  that key to the author's selected endpoint is a real credential boundary.
  Ambient project, workspace, and tenant keys alone do not authorize custom origins.
- The configured API/network peer can supply responses, redirects, status codes,
  and retry metadata, but should not redirect the credential across origins or
  force unbounded parsing.
- A pull-request author can propose changes and cause ordinary CI to execute the
  proposed checkout with read-only repository permission, but does not thereby
  receive protected release or publish credentials.
- Code-owner review and required checks authorize a merged release PR; the SDK
  App promotes its exact revision into release workflows. Their configured GitHub protections
  are external to this checkout.

### Canonical repository-code trust rule

For scans of a pinned revision, the reviewed, tracked checkout is the
repository-code authority under analysis. That includes intentionally
executable source, generated code, examples, tests, fixtures, build scripts,
generator inputs, local composite actions, and workflow scripts. A contributor
who can modify those tracked files in a proposed PR does not gain a new
privilege merely because ordinary CI runs that proposed code; this is expected
repository-code execution under the workflow's existing authority. Generated
ownership markers describe maintenance ownership, not a lower runtime trust
class ([CONTRIBUTING.md](../../CONTRIBUTING.md#L32-L43),
[.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15),
[.github/workflows/ci.yml](../../.github/workflows/ci.yml#L43-L162)).

This rule is not a blanket suppression. Treat a path as a real boundary when an
independently mutable lower-trust value crosses a parser, evaluator, network,
filesystem, credential, authorization, or publication boundary; when untrusted
runtime/API/network data reaches a sensitive sink; or when proposed PR code can
reach credentials or authority reserved for protected CI/release workflows.

### Boundary crossings and invariants

1. **Terraform host to provider.** Configuration, environment variables, plan,
   state, and import IDs enter privileged API operations. All four credential
   fields are sensitive, configuration precedence is explicit, and each
   resource fails configuration when its own audience credential is missing;
   one audience never substitutes for another
   ([internal/provider/provider.go](../../internal/provider/provider.go#L106-L177)).
2. **Provider to API origin.** A custom endpoint is intentional only when the
   same trusted authority selects the destination and supplies or authorizes
   the applicable credential. If a lower-trust Terraform config author selects
   `base_url` while a separate runtime injects `OPENAI_ADMIN_KEY` or supplies an
   explicit credential attribute, credential forwarding is a real boundary.
   Ambient project and tenant keys are bound to `https://api.openai.com/v1`;
   the ambient workspace key is bound to `https://api.chatgpt.com/v1`. Custom
   origins require the corresponding credential attribute to be configured
   explicitly. Every endpoint must be an
   absolute HTTP(S) URL without
   embedded credentials; plaintext is loopback-only, and redirects cannot cross
   the configured origin or exceed ten hops
   ([internal/provider/provider_security.go](../../internal/provider/provider_security.go#L19-L52),
   [internal/provider/provider_security.go](../../internal/provider/provider_security.go#L98-L117)).
3. **Identifiers to URL paths.** Independently mutable IDs must not alter
   route structure. Empty and dot-segment values are rejected, unresolved
   placeholders fail, and values are path-escaped
   ([internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L410-L435)).
4. **API response to Terraform state.** Response bodies and pagination are
   lower-trust network data. Per-response and aggregate byte budgets and page
   limits bound GET and paginated data-source parsing and work; typed extraction
   validates modeled fields, while data sources intentionally serialize the
   bounded raw GET map into `response_json`. That raw state can contain
   API-returned fields beyond the modeled attributes and remains potentially
   confidential. Mutation responses use the SDK decoder under overall request
   timeout and retry controls, but do not currently share the repository's GET
   byte budgets
   ([internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L337-L402),
   [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L870-L929),
   [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L946-L1000),
   [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L1071-L1144),
   [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L1450-L1456),
   [internal/provider/resources/certificate/data_source_certificate.go](../../internal/provider/resources/certificate/data_source_certificate.go#L92-L98),
   [internal/provider/resources/certificate/data_source_certificate.go](../../internal/provider/resources/certificate/data_source_certificate.go#L155-L156)).
   The webhook signing secret is a special one-time create response: it is
   stored as sensitive state, preserved when absent from reads and updates, and
   remains unavailable after import.
5. **Acceptance tests to real organizations, workspaces, and tenants.** Acceptance tests are a separate,
   mutation-capable workflow and must remain explicitly opted in with
   `TF_ACC=1`, the applicable audience credential, and fixture values; ordinary
   unit and CI runs must not silently become live administration, webhook, or WIF
   mutation
   ([CONTRIBUTING.md](../../CONTRIBUTING.md#L168-L173)).
6. **PR CI to protected release authority.** Proposed code running in ordinary
   CI is expected under read-only repository permission. It becomes a finding
   if that code can access protected `release` or `publish` credentials,
   write-capable tokens, signing keys, or publication authority
   ([.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15),
   [.github/workflows/release-please.yml](../../.github/workflows/release-please.yml#L8-L26),
   [.github/workflows/release.yml](../../.github/workflows/release.yml#L26-L33)).
7. **Tag to signed publication.** A `v*` tag triggers the Release workflow,
   but reaches the privileged publication job only after release-SBOM
   verification succeeds and the tag matches the exact merged release PR. The
   publish environment restricts credentials to release tags; no second manual
   deployment approval is required.
   Downloaded tools, ambient runner state, dependency caches, generated SBOMs,
   checksums, and attestations remain distinct lower-trust inputs that must be
   verified before signing or publication
   ([.github/workflows/release.yml](../../.github/workflows/release.yml#L12-L109),
   [.github/actions/setup-release-tools/install.sh](../../.github/actions/setup-release-tools/install.sh#L28-L57),
   [.github/actions/verify-release-dependencies/trusted-release-environment.sh](../../.github/actions/verify-release-dependencies/trusted-release-environment.sh#L3-L68)).

### Assumptions and unknowns

- OpenAI API authentication, authorization, tenant isolation, and mutation
  semantics are external service controls; this repository invokes them but
  cannot prove them offline.
- Terraform host integrity, state-backend access control, proxy/TLS trust roots,
  and environment-variable provenance are deployment responsibilities.
- Branch protection, tag creation/immutability rules, environment ref filters, and secret
  scoping are documented operational expectations but are not verifiable from
  this checkout ([RELEASING.md](../../RELEASING.md#L80-L85)).
- Upstream generator provenance is not present here. Scans should analyze the
  checked-in generated output as reviewed executable repository code while
  preserving generator ownership constraints
  ([CONTRIBUTING.md](../../CONTRIBUTING.md#L32-L49)).

## 3. Attack Surface, Mitigations, and Attacker Stories

These are review hypotheses and calibration examples, not confirmed findings.

| Priority | Scenario and capability gain | Prerequisites | Impact | Existing controls | Mitigation | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| High | A lower-trust redirect, endpoint confusion, or Terraform configuration sends an independently injected credential to an unintended origin. | Attacker influences redirect metadata or endpoint parsing, or controls `base_url` while a separate trusted runtime supplies `OPENAI_ADMIN_KEY` or an explicitly configured credential attribute. | Credential disclosure and the associated organization- or project-level API authority. | URL validation rejects userinfo/non-loopback HTTP; redirects stay on the canonical configured origin and are capped; `OPENAI_API_KEY` is bound to the default OpenAI origin; resources select an explicit credential audience without fallback. These controls cannot establish whether the config author is trusted to pair an explicit credential with a destination. | Preserve audience isolation, ambient project-key origin binding, origin canonicalization, redirect checks, redacted errors, and deployment-level alignment between endpoint selection and explicitly configured credential authority. | [internal/provider/provider.go](../../internal/provider/provider.go#L113-L181), [internal/provider/provider_security.go](../../internal/provider/provider_security.go#L19-L52), [internal/provider/provider_security.go](../../internal/provider/provider_security.go#L77-L121), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L530-L549) |
| High | Proposed PR code reaches protected release credentials or publication authority from ordinary CI. | Workflow change, token scope change, or environment exposure bridges normal PR execution to protected authority. | Signing-key theft or unauthorized artifact publication. | Ordinary CI is read-only; release and publish are separate protected workflows/environments. | Preserve least privilege, environment separation, pinned actions, and review changes to workflows as security-sensitive. | [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15), [.github/workflows/release-please.yml](../../.github/workflows/release-please.yml#L8-L26), [.github/workflows/release.yml](../../.github/workflows/release.yml#L26-L33) |
| High | Unverified runner state, downloaded tools, dependencies, or SBOM output contaminates a signed release. | Attacker controls a mutable release input after review but before signing. | Malicious or misdescribed signed provider artifacts. | Pinned tool digests, clean module/build caches, `go mod verify`, sanitized Go/Git environment, SBOM verification, signed checksums, and attestations. | Keep verification before credential import/signing and fail closed on drift. | [.github/actions/setup-release-tools/install.sh](../../.github/actions/setup-release-tools/install.sh#L28-L57), [.github/actions/verify-release-dependencies/action.yml](../../.github/actions/verify-release-dependencies/action.yml#L7-L22), [.github/workflows/release.yml](../../.github/workflows/release.yml#L44-L109) |
| Medium | Crafted config, state, or import identifiers alter an API route or address another resource. | Lower-trust identifier reaches path construction. | Wrong-resource read or mutation under the operator's credential. | Empty/dot segments are rejected and path components are escaped. | Preserve validators and import parsing for every identifier-bearing resource. | [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L410-L435), [internal/provider/resources/project_service_account/resource_project_service_account.go](../../internal/provider/resources/project_service_account/resource_project_service_account.go#L247-L263) |
| Medium | Malformed, oversized, or adversarially paginated API data causes unsafe state changes or resource exhaustion. | Configured API/network peer returns hostile responses. | Bounded denial of service or invalid state; API-side semantic authorization remains external. | GET/paginated responses have byte and page budgets, cursor checks, and typed extraction; all methods have overall timeout and bounded retries, while mutation responses do not currently share the GET byte budgets. | Preserve GET/pagination bounds, typed decoding, and request-lifecycle controls; assess mutation-response size handling when changing that path. | [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L337-L402), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L717-L747), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L870-L929), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L946-L1000), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L1071-L1144) |
| Medium | A credential, webhook signing secret, or other sensitive API value leaks through state, raw `response_json`, diagnostics, logs, fixtures, or artifacts. | Sensitive value reaches an observable sink available to a less-privileged actor. | Credential disclosure or confidential organization/project data exposure; a leaked signing secret enables forged webhook signatures for that endpoint. | Sensitive schema marking, redaction guidance, and lifecycle telemetry that records request metadata rather than bodies; the signing secret is read only from creation and is never sent in provider mutation bodies; data sources intentionally persist bounded raw response JSON. | Keep diagnostics and telemetry body/header-free; protect state, especially one-time signing secrets, and treat raw response JSON as potentially confidential; use synthetic test data. | [internal/provider/provider.go](../../internal/provider/provider.go#L67-L84), [internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go](../../internal/provider/resources/webhook_endpoint/resource_webhook_endpoint.go#L116-L126), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L1450-L1456), [CONTRIBUTING.md](../../CONTRIBUTING.md#L76-L93), [internal/provider/openaiapi/client.go](../../internal/provider/openaiapi/client.go#L486-L509) |
| Medium | Acceptance tests unexpectedly mutate a real organization. | Live credentials and fixture identifiers are present and opt-in gating is bypassed or weakened. | Unintended organization, project, role, retention, or spend-control mutation. | Documented explicit opt-in and fixture requirements. | Preserve acceptance gating and never enable it in ordinary CI. | [CONTRIBUTING.md](../../CONTRIBUTING.md#L168-L173) |
| Low / not a new capability by itself | A contributor changes a checked-in test, fixture, example, build script, generated file, or workflow script and ordinary CI executes it. | Contributor can already propose tracked repository-code changes; no bridge to protected authority exists. | Expected execution of proposed repository code under ordinary CI's existing read-only authority. | Canonical repository-code trust rule and read-only CI permissions. | Report only a concrete boundary crossing, such as protected credentials, independent lower-trust input at a sensitive sink, or privilege expansion. | [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L13-L15), [.github/workflows/ci.yml](../../.github/workflows/ci.yml#L43-L162), [CONTRIBUTING.md](../../CONTRIBUTING.md#L32-L43) |

## 4. Severity Calibration

- **Critical:** A realistic unauthenticated or lower-privilege actor obtains
  broad organization-admin authority, protected signing keys, or unauthorized
  publication of trusted artifacts. Cross-tenant API compromise is critical
  only when the provider actually bypasses an independently enforced API
  authorization boundary.
- **High:** A lower-trust runtime/network input leaks an API or workspace admin
  credential, crosses the configured-origin or audience-isolation restriction,
  lets ordinary PR execution reach protected release credentials, or
  contaminates signed artifacts despite the release controls.
- **Medium:** A reachable boundary failure causes bounded wrong-resource
  mutation, confidential state/log exposure, malformed-response state effects,
  or production-relevant denial of service with meaningful prerequisites.
- **Low:** A limited robustness issue has no demonstrated sensitive sink or
  meaningful new authority. Executing intentionally tracked repository code in
  ordinary CI, custom HTTPS destinations selected by the same trusted authority
  that supplies or authorizes the key, authorized Admin or project API
  operations, and hypotheses without a realistic lower-trust route are not
  findings by themselves.

Severity changes with actual attacker starting capability, reachable sensitive
sink, API-side authorization, deployment configuration, workflow permissions,
and effective controls. Missing evidence is an open question, not proof of
either safety or vulnerability.
