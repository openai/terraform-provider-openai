resource "openai_project_user" "example" {
  project_id = "proj_123"
  role       = "owner"
  user_id    = "user_123"
}
