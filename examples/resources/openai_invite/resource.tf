resource "openai_invite" "example" {
  email    = "terraform@example.com"
  role     = "owner"
  projects = []
}
