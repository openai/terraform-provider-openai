resource "openai_organization_spend_alert" "example" {
  threshold_amount                = 20
  currency                        = "USD"
  interval                        = "month"
  notification_channel_type       = "email"
  notification_channel_recipients = ["terraform@example.com"]
}
