# Secure MCP tunnels

Use `openai_mcp_tunnel` in the official `openai/openai` provider to register a
private MCP server, update its name and description, or adopt an existing tunnel.
Run `tunnel-client` in a network that can reach the private MCP server and give
it the tunnel ID returned by Terraform.

## Minimal configuration

Supply the administrator credential through `OPENAI_ADMIN_KEY`. The principal
needs Tunnels Read + Manage in the intended Platform organization.
The service attaches the caller's active organization and workspace when
present in the authenticated context. The resource reports those
associations as computed attributes; it does not configure or reconcile them.

```terraform
terraform {
  required_providers {
    openai = {
      source  = "openai/openai"
      version = ">= 1.2.0"
    }
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

With `OPENAI_ADMIN_KEY` set for the intended organization, initialize and apply
the configuration:

```sh
terraform init
terraform validate
terraform plan
terraform apply
terraform output -raw tunnel_id
```

If upgrading an existing configuration, run `terraform init -upgrade` to replace
an older provider selection. If your organization configures a provider mirror,
use its approved installation path. Review the plan and confirm the apply when
prompted.

Use the returned ID as `CONTROL_PLANE_TUNNEL_ID` in your runtime, or as
`tunnel_id` in a Responses API MCP tool. Tunnel registration does not prove that
the runtime is connected or the private MCP server is healthy.

Both `name` and `description` are required. Set `description = ""` to clear the
description. Changing either field updates the existing registration without
replacing its ID.

To read a registration managed elsewhere, use the data source:

```terraform
data "openai_mcp_tunnel" "existing" {
  id = "tunnel_0123456789abcdef0123456789abcdef"
}
```

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

## Use a local development build

To test changes from a provider checkout, build it to a local directory:

```sh
mkdir -p /tmp/openai-tunnel-provider
go build -o /tmp/openai-tunnel-provider/terraform-provider-openai .
```

Create `/tmp/openai-tunnel-provider.tfrc` with the following Terraform CLI
configuration:

```hcl
provider_installation {
  dev_overrides {
    "openai/openai" = "/tmp/openai-tunnel-provider"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file, then run Terraform in the directory
containing the minimal configuration above:

```sh
export TF_CLI_CONFIG_FILE=/tmp/openai-tunnel-provider.tfrc
terraform validate
terraform plan
terraform apply
terraform output -raw tunnel_id
```

Ensure `OPENAI_ADMIN_KEY` is set for the intended organization before running
the plan. Review the plan and confirm the apply when prompted. For a
configuration using only the local OpenAI provider, run these commands directly;
`terraform init` resolves published versions and does not install this local
build. The Kubernetes example also requires its Kubernetes provider to be
initialized as described in its README.

## Adopt or remove a registration

To adopt an existing registration, add its resource configuration and import
its tunnel ID:

```sh
terraform import openai_mcp_tunnel.private_mcp tunnel_0123456789abcdef0123456789abcdef
```

Import adopts an existing registration. Destroy deletes the registration and
interrupts clients and consumers using its ID. Terraform resource dependencies
order removal of the example deployment before the tunnel.

## Validate against an isolated account

The provider includes an opt-in acceptance test that creates, updates, reads,
imports, and deletes a tunnel. Set `OPENAI_ADMIN_KEY` for an authorized isolated
organization, then run this command from the provider checkout:

```sh
TF_ACC=1 go test ./internal/provider/resources/mcp_tunnel -run '^TestAccMCPTunnel_Lifecycle$' -v
```

Use `OPENAI_TF_ACC_BASE_URL` when testing against a separate API environment.
This checks registration management; validate the running tunnel client and MCP
server separately before connecting consumers.

## Scope

Configure runtime process management, runtime key issuance, private-connectivity
credentials, client-network policies, app registration, and upstream OAuth with
their respective tools. A tunnel associated only with a Platform organization
may need a ChatGPT workspace association before it appears in that workspace.

See the [Secure MCP Tunnel guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)
and [tunnel-client](https://github.com/openai/tunnel-client) for permissions,
deployment, and runtime configuration.
