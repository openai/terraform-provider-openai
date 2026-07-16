resource "openai_project_role" "example" {
  project_id  = "proj_123"
  role_name   = "Terraform managed role"
  permissions = ["api.project.read"]
}
