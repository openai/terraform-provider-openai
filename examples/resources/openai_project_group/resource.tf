resource "openai_project_group" "example" {
  project_id = "proj_123"
  group_id   = "group_123"
  role       = "role-api-project-member"
}
