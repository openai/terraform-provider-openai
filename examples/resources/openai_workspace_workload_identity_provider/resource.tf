resource "openai_workspace_workload_identity_provider" "example" {
  workspace_id              = "workspace_123"
  name                      = "ci"
  issuer                    = "https://token.actions.githubusercontent.com"
  attribute_transformations = []
}
