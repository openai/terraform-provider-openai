resource "openai_project_spend_alert" "example" {
  project_id                      = "proj_123"
  threshold_amount                = 20
  currency                        = "USD"
  interval                        = "month"
  notification_channel_type       = "email"
  notification_channel_recipients = ["api.organization.read"]
}
