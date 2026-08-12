resource "openai_organization_spend_limit" "example" {
  threshold_amount = 20
  currency         = "USD"
  interval         = "month"
}
