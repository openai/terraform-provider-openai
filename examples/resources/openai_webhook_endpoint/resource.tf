resource "openai_webhook_endpoint" "example" {
  name        = "production-events"
  url         = "https://example.com/openai/webhooks"
  event_types = ["response.completed"]
}
