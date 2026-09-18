variable "name" {
  type        = string
  default     = "private-mcp-tunnel"
  description = "Tunnel, Kubernetes Deployment, and app label name. Use a lowercase DNS label of at most 63 characters."

  validation {
    condition     = length(var.name) <= 63 && can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?$", var.name))
    error_message = "Use 1-63 lowercase letters, digits, or hyphens, starting and ending with a letter or digit."
  }
}

variable "namespace" {
  type        = string
  description = "Existing namespace containing the runtime Secret."
}

variable "kubeconfig_path" {
  type        = string
  description = "Path to your kubeconfig file."
}

variable "kubeconfig_context" {
  type        = string
  description = "Kubeconfig context for the target cluster."
}

variable "tunnel_client_image" {
  type        = string
  description = "Reviewed tunnel-client container image pinned by digest, for example ghcr.io/openai/tunnel-client@sha256:<digest>."

  validation {
    condition     = can(regex("@sha256:[0-9a-f]{64}$", var.tunnel_client_image))
    error_message = "Pin the tunnel-client image to a SHA-256 digest."
  }
}

variable "mcp_server_url" {
  type        = string
  description = "Private HTTP MCP endpoint reachable from the Pod, for example http://mcp.default.svc.cluster.local:8000/mcp."
}

variable "runtime_secret_name" {
  type        = string
  description = "Existing Kubernetes Secret containing a runtime API key with Tunnels Read + Use."
}

variable "runtime_secret_key" {
  type        = string
  default     = "api-key"
  description = "Key within the existing Secret. Terraform does not read the credential."
}
