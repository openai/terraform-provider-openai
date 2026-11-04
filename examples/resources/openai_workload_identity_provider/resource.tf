resource "openai_workload_identity_provider" "example" {
  name                      = "terraform-managed"
  issuer                    = "https://issuer.example.com"
  audience                  = "openai"
  attribute_transformations = []
}
