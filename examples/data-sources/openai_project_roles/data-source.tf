data "openai_project_roles" "available" {
  project_id = var.project_id
}

locals {
  member_role_id = one([
    for role in data.openai_project_roles.available.roles :
    role.id
    if role.predefined_role && role.name == "member"
  ])
}
