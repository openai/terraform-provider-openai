resource "openai_project_rate_limits" "example" {
  project_id  = "proj_123"
  rate_limits = { "gpt-4o" = { max_requests_per_1_minute = 20, max_tokens_per_1_minute = 20, max_images_per_1_minute = 20, max_audio_megabytes_per_1_minute = 20, max_requests_per_1_day = 20, batch_1_day_max_input_tokens = 20 } }
}
