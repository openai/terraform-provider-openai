resource "openai_project_rate_limits" "example" {
  project_id  = "proj_123"
  rate_limits = { "gpt-4o" = { max_tokens_per_1_minute = 20 } }
}
