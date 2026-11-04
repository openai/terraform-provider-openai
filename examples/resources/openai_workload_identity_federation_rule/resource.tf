resource "openai_workload_identity_federation_rule" "example" {
  workload_identity_provider_id = "workload_identity_provider_123"
  project_id                    = "proj_123"
  service_account_id            = "service_account_123"
  attribute_matchers            = { repository = "openai/example", protected = true, repository_id = 9007199254740993 }
}
