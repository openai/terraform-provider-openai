# Workload identity federation

Workload identity federation (WIF) lets an external workload authenticate as
an existing OpenAI principal. Terraform manages the identity provider's trust
settings and the rules that map external identities to principals. It does not
create those principals or perform token exchange.

Workspace and tenant WIF support is beta and requires the feature to be enabled
for the workspace or tenant.

## Choose the owner

Choose resources based on who owns the trust settings:

| Owner | Provider resource | Rule resource | Rule destination |
| --- | --- | --- | --- |
| API organization | `openai_workload_identity_provider` | `openai_workload_identity_federation_rule` | A project service account in that organization |
| ChatGPT workspace | `openai_workspace_workload_identity_provider` | `openai_workspace_workload_identity_federation_rule` | An existing principal in that workspace, for Codex |
| Tenant | `openai_tenant_workload_identity_provider` | `openai_tenant_workload_identity_federation_rule` | A linked ChatGPT workspace or API organization |

Existing Platform resource names, configuration, and import IDs keep working.
Use the new resource names when the workspace or tenant owns the provider.
Import does not change ownership.

Each tenant rule has exactly one `workspace_target` or
`api_organization_target`. The destination must already be linked to the
tenant. A tenant provider currently supports multiple ChatGPT workspaces and
one API organization.

## Credentials

Supply only the credentials needed by the resources in your configuration:

| Scope | Environment variable | Required authority |
| --- | --- | --- |
| API organization | `OPENAI_ADMIN_KEY` | Organization Admin API key |
| ChatGPT workspace | `OPENAI_WORKSPACE_ADMIN_KEY` | Workspace admin PAT with `workspace.workload_identity.management` |
| Tenant | `OPENAI_TENANT_ADMIN_KEY` | Tenant admin API key with `tenant.workload_identity.read` and, for resources, `tenant.workload_identity.write` |

Data sources need read access. Set the credentials in the Terraform process
environment for both plan and apply. A configuration that manages several
scopes can use the same provider instance; each resource selects its own
credential and endpoint. A missing credential produces an error for that
scope.

```terraform
provider "openai" {}
```

Workspace requests use `https://api.chatgpt.com/v1`. Tenant and Platform
requests use `https://api.openai.com/v1`. Override these with
`workspace_base_url`, `tenant_base_url`, and `base_url`, respectively. A custom
workspace or tenant origin requires an explicitly configured credential in
`workspace_admin_api_key` or `tenant_admin_api_key`; environment credentials
are bound to their default origins. Explicit credentials can be saved in a
Terraform plan, even when marked sensitive. On Terraform 1.10 or later, use an
ephemeral sensitive variable when you need an explicit credential.

## Workspace example

This example trusts a GitHub Actions environment and maps its subject to an
existing workspace principal. Replace the workspace, principal, repository,
and audience with your values.

```terraform
variable "workspace_id" {
  type = string
}

variable "principal_id" {
  type = string
}

resource "openai_workspace_workload_identity_provider" "github" {
  workspace_id = var.workspace_id
  name         = "github-actions"
  issuer       = "https://token.actions.githubusercontent.com"
  audience     = "https://openai.example.com/codex-ci"
  verification = { type = "discovery" }
}

resource "openai_workspace_workload_identity_federation_rule" "deploy" {
  workspace_id                  = var.workspace_id
  workload_identity_provider_id = openai_workspace_workload_identity_provider.github.id
  principal_id                  = var.principal_id
  external_subject              = "repo:example/codex-ci:environment:production"
  audiences                     = ["https://openai.example.com/codex-ci"]
}
```

Terraform attaches a workspace provider to its workspace and resolves the
Codex binding needed by each rule. There is no binding resource to configure.

## Tenant examples

A tenant can share a provider across its linked workspaces. The tenant is
selected by the tenant admin API key.

```terraform
resource "openai_tenant_workload_identity_provider" "github" {
  name         = "github-actions"
  issuer       = "https://token.actions.githubusercontent.com"
  verification = { type = "discovery" }
}

resource "openai_tenant_workload_identity_federation_rule" "deploy" {
  workload_identity_provider_id = openai_tenant_workload_identity_provider.github.id

  workspace_target = {
    workspace_id     = var.workspace_id
    principal_id     = var.principal_id
    external_subject = "repo:example/codex-ci:environment:production"
    audiences        = ["https://openai.example.com/codex-ci"]
  }
}
```

For an API organization, use `api_organization_target` with an existing project
service account. The referenced tenant provider must have trust settings and
an audience suitable for that workload.

```terraform
resource "openai_tenant_workload_identity_federation_rule" "api_job" {
  workload_identity_provider_id = var.tenant_provider_id

  api_organization_target = {
    organization_id    = var.organization_id
    project_id         = var.project_id
    service_account_id = var.service_account_id
    attribute_matchers = {
      sub = "repo:example/api-job:environment:production"
    }
    permissions = { type = "service_account_default" }
  }
}
```

Use `permissions = { type = "restricted", scopes = [...] }` to request a
restricted set of API permissions. This requires a nonempty scope list.

## Reads and imports

Each new scope has four data sources: one provider, a collection of providers,
one rule, and a collection of rules. Their names match the resources, with
`providers` and `federation_rules` for the collection forms. Individual reads
expose the full configuration. Collection reads expose typed summaries and
the complete API result in `response_json`.

Workspace data sources can read tenant-owned providers assigned to that
workspace. Manage those providers through the tenant resources.

| Resource | Import ID |
| --- | --- |
| Workspace provider | `workspace_id/provider_id` |
| Workspace rule | `workspace_id/provider_id/rule_id` |
| Tenant provider | `provider_id` |
| Tenant rule | `provider_id/rule_id` |

After importing, write configuration that matches the returned settings and
review the plan before applying it.

## Updates and removal

For workspace and tenant rules, changing the destination or principal replaces
the rule. Changing its subject, claims, condition, or permissions updates it in
place. The resource reference from a rule to its provider also gives Terraform
the dependency needed to remove rules before their provider.

Trust settings affect every rule that uses the provider. Review changes to
issuers, verification keys, audiences, and conditions before applying them.
The workspace provider supports OIDC discovery, uploaded JWKS, and a JWKS URI.
Tenant providers also support a custom discovery URL. JWKS values are JSON
objects containing public keys; do not supply private signing keys.

If creating a workspace provider succeeds but attaching it fails, Terraform
retains its ID so a later apply can clean up and replace the incomplete
resource. Refresh does not create bindings. Authentication and permission
errors remain errors and do not remove resources from state.
