# Prefer environment variables for local use, for example:
# export OPENAI_ADMIN_KEY=...
# export OPENAI_API_KEY=...
# export OPENAI_ORG_ID=...
# export OPENAI_PROJECT_ID=...

variable "admin_api_key" {
  type        = string
  description = "Admin API key used for organization administration requests."
  sensitive   = true
  default     = null
}

variable "api_key" {
  type        = string
  description = "Project API key used for project-scoped requests such as webhook management."
  sensitive   = true
  default     = null
}

provider "openai" {
  # You can omit this argument when OPENAI_ADMIN_KEY is set.
  admin_api_key = var.admin_api_key
  # You can omit this argument when OPENAI_API_KEY is set.
  api_key = var.api_key
  # organization can also be set with OPENAI_ORG_ID.
  # project can also be set with OPENAI_PROJECT_ID.
}
