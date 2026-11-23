resource "openai_tenant_workload_identity_provider" "example" {
  name                      = "ci"
  issuer                    = "https://token.actions.githubusercontent.com"
  verification              = { type = "discovery" }
  attribute_transformations = []
}
