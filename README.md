# Terraform Provider for OpenAI

The OpenAI Terraform provider gives Terraform configurations convenient access
to the [OpenAI Administration API](https://developers.openai.com/api/reference/administration/overview).
Use it to manage organization-level resources such as projects, users, groups,
roles, service accounts, certificates, rate limits, spend alerts, and related
project settings.

See [`docs/`](docs/index.md) for resource and data source documentation.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) CLI 1.0 or later
- An [OpenAI Admin API key](https://platform.openai.com/settings/organization/admin-keys)

[Admin API keys](https://developers.openai.com/api/docs/guides/admin-apis) are
required for Administration API endpoints and cannot be used for
non-administration OpenAI API endpoints.

## Usage

Set your Admin API key in the environment:

```sh
export OPENAI_ADMIN_KEY="<your-admin-api-key>"
```

Then create a Terraform configuration:

```terraform
terraform {
  required_version = ">= 1.0"

  required_providers {
    openai = {
      source = "openai/openai"
    }
  }
}

provider "openai" {
  # The provider reads OPENAI_ADMIN_KEY by default.
  # You can also set admin_api_key, organization, and project here.
}

resource "openai_project" "example" {
  name = "terraform-managed"
}
```

See [`docs/index.md`](docs/index.md) for provider configuration details and the
full resource and data source documentation.

## License

This project is licensed under the [Apache License 2.0](LICENSE).
