# Terraform Provider for OpenAI

The OpenAI Terraform provider manages resources across four API audiences:

- The [OpenAI Administration API](https://developers.openai.com/api/reference/administration/overview)
  for organization resources such as projects, users, groups, roles, service
  accounts, certificates, rate limits, and spend alerts.
- The project-scoped webhook API for webhook endpoints and event-type discovery.
- The workspace API for Codex workload identity providers and federation rules.
- The tenant API for shared workload identity providers and rules that target
  ChatGPT workspaces or API organizations.

Webhook management is **beta**. Its API and Terraform schema may change before
general availability. Organization-level webhook management is not supported.
Workspace and tenant workload identity management are also **beta**. See the
[workload identity guide](docs/guides/workload-identity.md) for credentials,
examples, and the choice of resource scope.

See [`docs/`](docs/index.md) for resource and data source documentation.
For private MCP servers, see the [Secure MCP tunnel guide](docs/guides/mcp-tunnel.md)
and its Kubernetes deployment example.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) CLI 1.0 or later
- The credentials below for the resources being managed

| Terraform operations | Provider attribute | Environment variable | Credential |
| --- | --- | --- | --- |
| Organization administration, including every `openai_project*` resource and data source | `admin_api_key` | `OPENAI_ADMIN_KEY` | [OpenAI Admin API key](https://platform.openai.com/settings/organization/admin-keys) |
| Beta project webhook resources and data sources | `api_key` | `OPENAI_API_KEY` | Project API key with `api.webhooks.read`; mutations also require `api.webhooks.write` and project-owner authority |
| Workspace workload identity providers and federation rules | `workspace_admin_api_key` | `OPENAI_WORKSPACE_ADMIN_KEY` | Workspace admin personal access token with `chatgpt.workspace.workload_identity.write` |
| Tenant workload identity providers and federation rules | `tenant_admin_api_key` | `OPENAI_TENANT_ADMIN_KEY` | Tenant admin API key with `tenant.workload_identity.read`; mutations also require `tenant.workload_identity.write` |

[Admin API keys](https://developers.openai.com/api/docs/guides/admin-apis)
cannot be used for non-administration endpoints. These credentials are not
substitutes for one another.

## Usage

Set the credentials required by your configuration:

```sh
export OPENAI_ADMIN_KEY="<your-admin-api-key>"
export OPENAI_API_KEY="<your-project-api-key>"
export OPENAI_WORKSPACE_ADMIN_KEY="<your-workspace-admin-pat>"
export OPENAI_TENANT_ADMIN_KEY="<your-tenant-admin-api-key>"
```

Set only the credentials your configuration needs. For example, this
configuration needs only `OPENAI_ADMIN_KEY`:

```terraform
terraform {
  required_version = ">= 1.0"

  required_providers {
    openai = {
      source = "openai/openai"
    }
  }
}

provider "openai" {
  # The provider reads audience-specific credentials from the environment.
}

resource "openai_project" "example" {
  name = "terraform-managed"
}
```

Supply `OPENAI_ADMIN_KEY` to the Terraform process for both `terraform plan`
and `terraform apply`, including when applying a saved plan on another runner.
Configure each runner to inject the credential into its process environment.

Explicit `admin_api_key` configuration remains supported, but it is deprecated:
ordinary Terraform variables, even `sensitive` ones, can be retained in a saved
plan. To migrate, remove the Admin key variable and provider assignment, and use
`OPENAI_ADMIN_KEY` for each run. On Terraform 1.10 or later, an ephemeral
sensitive input is also an option; see [provider configuration](docs/index.md).
The environment-only example above continues to work with Terraform 1.0.

A webhook-only configuration does not require an Admin API key:

```terraform
provider "openai" {}

resource "openai_webhook_endpoint" "events" {
  name        = "production-events"
  url         = "https://example.com/openai/webhooks"
  event_types = ["response.completed"]
}
```

Credentials are routed only to their configured audience. If the required
credential is absent, the affected resource or data source reports the exact
provider attribute and environment variable to set. The provider cannot infer
a key's type from its value: putting a project key in `admin_api_key`, or an
Admin key in `api_key`, sends it only to that field's API audience, where the
OpenAI API rejects it. The provider never retries with the other credential.

The `project` provider attribute, or `OPENAI_PROJECT_ID`, sends the
`OpenAI-Project` header for credentials that support project selection. It does
not grant project access or change a credential's audience.

`OPENAI_API_KEY` is used only with the default OpenAI API origin. A custom
`base_url` requires an explicit `api_key` to authorize sending the project
credential there. For backward compatibility, `OPENAI_ADMIN_KEY` remains
eligible for a configured custom origin; only trusted Terraform configuration
should choose `base_url` when an Admin key is injected by the runtime.

Workspace and tenant requests use separate endpoints: `workspace_base_url`
defaults to `https://api.chatgpt.com/v1`, and `tenant_base_url` defaults to
`https://api.openai.com/v1`. Each environment credential is used only with its
default origin. A custom endpoint requires an explicit credential in the
matching provider attribute. Workspace and tenant requests do not send the
Platform organization or project headers.

Webhook signing secrets are returned only during creation and are stored as
sensitive values in Terraform state. Protect the state backend. Import cannot
recover the secret, and rotating it outside Terraform makes the stored value
stale. Endpoint test delivery and signing-secret rotation are imperative API
operations and are not managed by this provider.

See [`docs/index.md`](docs/index.md) for provider configuration details and the
full resource and data source documentation.

## License

This project is licensed under the [Apache License 2.0](LICENSE).
