resource "openai_organization_user" "example" {
}

import {
  to = openai_organization_user.example
  id = "user_123"
}
