resource "openai_project_model_permissions" "example" {
  project_id = "proj_123"
  mode       = "allow_list"
  model_ids  = ["api.organization.read"]
}
