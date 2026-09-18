terraform {
  required_version = ">= 1.0"

  required_providers {
    openai = {
      source  = "openai/openai"
      version = ">= 1.2.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
}

provider "openai" {
  # Reads OPENAI_ADMIN_KEY. This credential is never given to the container.
}

provider "kubernetes" {
  config_path    = var.kubeconfig_path
  config_context = var.kubeconfig_context
}

resource "openai_mcp_tunnel" "private_mcp" {
  name        = var.name
  description = "Private MCP gateway managed with Terraform"
}

resource "kubernetes_deployment_v1" "tunnel" {
  metadata {
    name      = var.name
    namespace = var.namespace
  }

  spec {
    replicas = 1

    # Keep this example to one active client, including during rollout.
    strategy {
      type = "Recreate"
    }

    selector {
      match_labels = { app = var.name }
    }

    template {
      metadata {
        labels = { app = var.name }
      }

      spec {
        automount_service_account_token = false

        container {
          name    = "tunnel-client"
          image   = var.tunnel_client_image
          command = ["/usr/bin/tunnel-client"]
          args    = ["run"]

          env {
            name  = "CONTROL_PLANE_TUNNEL_ID"
            value = openai_mcp_tunnel.private_mcp.id
          }

          env {
            name = "CONTROL_PLANE_API_KEY"
            value_from {
              secret_key_ref {
                name = var.runtime_secret_name
                key  = var.runtime_secret_key
              }
            }
          }

          env {
            name  = "MCP_SERVER_URL"
            value = var.mcp_server_url
          }

          env {
            name  = "HEALTH_LISTEN_ADDR"
            value = ":8080"
          }

          port {
            name           = "health"
            container_port = 8080
          }

          readiness_probe {
            http_get {
              path = "/readyz"
              port = "health"
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          liveness_probe {
            http_get {
              path = "/healthz"
              port = "health"
            }
            initial_delay_seconds = 15
            period_seconds        = 20
          }
        }
      }
    }
  }
}

output "tunnel_id" {
  value = openai_mcp_tunnel.private_mcp.id
}
