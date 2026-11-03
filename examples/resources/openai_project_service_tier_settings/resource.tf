resource "openai_project_service_tier_settings" "example" {
  project_id    = "proj_123"
  default       = "standard"
  enabled_tiers = "all"
}
