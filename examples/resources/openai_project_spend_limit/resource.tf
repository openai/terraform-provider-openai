resource "openai_project_spend_limit" "example" {
  project_id       = "proj_123"
  threshold_amount = 10000
  currency         = "USD"
  interval         = "month"
}
