data "openai_workspace_workload_identity_federation_rule" "example" {
  workspace_id                         = "workspace_123"
  workload_identity_provider_id        = "idp_123"
  workload_identity_federation_rule_id = "workload_identity_federation_rule_123"
}
