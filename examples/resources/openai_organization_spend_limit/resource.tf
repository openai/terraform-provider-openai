resource "openai_organization_spend_limit" "example" {
  threshold_amount = 10000
  currency         = "USD"
  interval         = "month"
}
