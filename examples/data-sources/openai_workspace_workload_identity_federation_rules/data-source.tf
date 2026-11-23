data "openai_workspace_workload_identity_federation_rules" "example" {
  workspace_id                  = "workspace_123"
  workload_identity_provider_id = "idp_123"
}
