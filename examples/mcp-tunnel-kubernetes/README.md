# Secure MCP Tunnel on Kubernetes

This example requires the unreleased `openai_mcp_tunnel` prototype. Follow the
[local provider instructions](../../docs/guides/mcp-tunnel-prototype.md) before
running it.

The OpenAI provider creates the tunnel registration. The Kubernetes provider
runs a single `tunnel-client` gateway that forwards to an existing private HTTP
MCP server. The namespace, MCP service, cluster access, and runtime Secret must
already exist.

Set `OPENAI_ADMIN_KEY` for Terraform. Supply the container a separate restricted
runtime API key with Tunnels Read + Use through the existing Secret. Only the
Secret's name and key are referenced in this configuration; no credential
value is stored in its Terraform variables or resource state.

Initialize the Kubernetes provider, then set the variables and inspect the
plan. Use a reviewed `ghcr.io/openai/tunnel-client` image digest. Applying this
example creates a real tunnel and Kubernetes Deployment. This example uses
one client with the `Recreate` strategy to avoid rollout overlap; upgrades
entail downtime. No Service or Ingress is necessary for the client; it needs
outbound HTTPS to OpenAI and access to the private MCP endpoint. Port 8080 is
used for Pod health probes, and the remote admin UI remains disabled.

If the MCP endpoint balances requests across several backends, those backends
must be stateless, share session state, or preserve MCP session affinity.
Using a single gateway does not provide that affinity. Multiple gateways can
also share a tunnel when the HTTP backend meets these conditions; see the
[tunnel-client deployment guide](https://github.com/openai/tunnel-client/blob/master/docs/deployment/kubernetes-dedicated.md#replicas-and-active-active-behavior).

## Sidecar variant

Use the same `openai_mcp_tunnel` resource and move the `tunnel-client` container
block into the existing MCP server's Pod template:

- Set `MCP_SERVER_URL` to the MCP server's loopback URL, such as
  `http://127.0.0.1:8000/mcp`.
- Keep the Secret reference and `CONTROL_PLANE_TUNNEL_ID` expression.
- Choose a health port that does not conflict with the MCP server.
- Keep one active client for this tunnel, including during rollouts. Do not
  copy the same tunnel ID into every replica of an independently scaled MCP
  Deployment. Use the dedicated gateway example when the MCP service scales
  independently.

The resource is identical for gateway and sidecar deployments. The difference
is where the client runs and how it reaches the MCP server.

After applying, check `/readyz` and exercise an MCP request through a supported
OpenAI product. A successful Terraform apply alone is not an end-to-end tunnel
test. Deleting the tunnel registration disconnects consumers of its ID.
