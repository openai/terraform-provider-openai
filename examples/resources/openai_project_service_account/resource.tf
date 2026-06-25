resource "openai_project_service_account" "example" {
  project_id = "proj_123"
  name       = "terraform-managed"
}
