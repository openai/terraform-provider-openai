resource "openai_role" "example" {
  role_name   = "Terraform managed role"
  permissions = ["api.organization.read"]
}
