//go:build generate

package tools

import _ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"

//go:generate go run -mod=readonly github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name openai
