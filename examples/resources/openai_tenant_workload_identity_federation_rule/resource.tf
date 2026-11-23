resource "openai_tenant_workload_identity_federation_rule" "example" {
  workload_identity_provider_id = "workload_identity_provider_123"
  workspace_target              = { workspace_id = "workspace_123", principal_id = "user_123", external_subject = "repo:example/codex-ci:environment:production", audiences = ["https://openai.example.com/codex-ci"] }
}
