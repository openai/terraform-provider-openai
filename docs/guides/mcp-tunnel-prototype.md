# Secure MCP Tunnel prototype

This is an unreleased prototype in the official `openai/openai` provider. It
manages the OpenAI tunnel registration; the customer runs `tunnel-client` in a
network that can reach their private MCP server.

## Minimal configuration

Supply the administrator credential through `OPENAI_ADMIN_KEY`. The principal
needs Tunnels Read + Manage in the intended Platform organization.
The service attaches the caller's active organization and workspace when
present in the authenticated context. This prototype reports those
associations as computed attributes; it does not configure or reconcile them.

```terraform
terraform {
  required_providers {
    openai = { source = "openai/openai" }
  }
}

provider "openai" {}

resource "openai_mcp_tunnel" "private_mcp" {
  name        = "private-mcp"
  description = "Private MCP server managed with Terraform"
}

output "tunnel_id" {
  value = openai_mcp_tunnel.private_mcp.id
}
```

Use the returned ID as `CONTROL_PLANE_TUNNEL_ID` in your runtime, or as
`tunnel_id` in a Responses API MCP tool. Tunnel registration does not prove that
the runtime is connected or the private MCP server is healthy.

## Deployment boundary

| Responsibility | Owner |
| --- | --- |
| Create, read, rename, describe, import, and delete a tunnel | OpenAI provider |
| Run the tunnel client and configure the private MCP target | Kubernetes, cloud, or host configuration |
| Issue and rotate a runtime API key | Existing credential workflow |
| Supply the runtime credential | Existing secret manager or Kubernetes Secret |
| Configure app access, workspace associations, and upstream OAuth | Existing OpenAI and MCP administration workflows |

The [Kubernetes example](../../examples/mcp-tunnel-kubernetes/README.md) shows a
dedicated gateway deployment and how to adapt it to a sidecar. Both use the same
OpenAI resource. ECS, VMs, and other container platforms can consume the same
tunnel ID without adding deployment-specific fields to the provider.

The runtime requires a separate restricted API key with Tunnels Read + Use.
Do not give it the provider's admin key. The example refers to an existing
Secret by name and key; it does not read or manage the Secret contents through
Terraform.

## Trying the local provider

Build this checkout to a local directory:

```sh
mkdir -p /tmp/openai-tunnel-provider
go build -o /tmp/openai-tunnel-provider/terraform-provider-openai .
```

Create a separate Terraform CLI configuration file for the prototype:

```hcl
provider_installation {
  dev_overrides {
    "openai/openai" = "/tmp/openai-tunnel-provider"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file for the prototype session. For a minimal
configuration using only the local OpenAI provider, use `terraform validate`
and `terraform plan` directly; `terraform init` resolves published versions and
does not install this local build. The Kubernetes example also requires its
Kubernetes provider to be initialized. Review a plan before applying against an
isolated organization.

```sh
terraform import openai_mcp_tunnel.private_mcp tunnel_0123456789abcdef0123456789abcdef
```

Import adopts an existing registration. Destroy deletes the registration and
interrupts clients and consumers using its ID. Terraform resource dependencies
order removal of the example deployment before the tunnel.

## Scope and release follow-up

This prototype excludes runtime process management, runtime key issuance,
private-connectivity credentials, client-network policies, app registration,
and upstream OAuth configuration. A tunnel associated only with a Platform
organization may need a ChatGPT workspace association before it appears in
that workspace.

Offline tests can establish Terraform lifecycle behavior against the API
contract. Before release, the SDK and tunnel owners should review the generated
change and validate the complete lifecycle in an authorized isolated account,
including a running tunnel client. No live account validation is implied by
the offline tests.

See the [Secure MCP Tunnel guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)
and [tunnel-client](https://github.com/openai/tunnel-client) for permissions,
deployment, and runtime configuration.
