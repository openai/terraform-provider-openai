resource "openai_workspace_workload_identity_federation_rule" "example" {
  workspace_id                  = "workspace_123"
  workload_identity_provider_id = "idp_123"
  principal_id                  = "user_123"
  external_subject              = "repo:example/codex-ci:environment:production"
}
