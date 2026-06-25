resource "openai_project_hosted_tool_permissions" "example" {
  project_id               = "proj_123"
  file_search_enabled      = true
  web_search_enabled       = true
  image_generation_enabled = true
  mcp_enabled              = true
  code_interpreter_enabled = true
}
