# Security Policy

## Reporting a vulnerability

Please report potential security vulnerabilities in the OpenAI Terraform Provider or its release artifacts privately by emailing disclosure@openai.com.

Do not report security vulnerabilities through public GitHub issues, pull requests, or discussions.

This policy applies to the OpenAI Terraform Provider and its release artifacts. For security issues in OpenAI services or other OpenAI products, use the same OpenAI disclosure process.

## What to include

Please include:

- The affected Terraform provider release, release artifact, or relevant source commit.
- A clear description of the vulnerability and its potential impact.
- Sanitized reproduction steps or a minimal proof of concept.
- When relevant, the Terraform version, operating system, architecture, and
  installation method.

Do not include live credentials, API keys, customer data, or unredacted sensitive logs.
Do not attach raw Terraform state, saved plans, crash output, or TF_LOG/provider
traces; provide only sanitized excerpts.

## Coordinated disclosure

Please give the maintainers a reasonable opportunity to investigate and address the issue before public disclosure.

Please coordinate respectfully with OpenAI's security team.

Please follow OpenAI's coordinated vulnerability disclosure policy:
https://openai.com/policies/coordinated-vulnerability-disclosure-policy

## Verifying release provenance

Terraform verifies the GPG-signed provider checksum when installing from the
Terraform Registry. Provider release archives also carry GitHub artifact
attestations linking their SHA-256 digests to this repository's protected
release workflow and the corresponding version tag.

After downloading a provider archive, verify its build provenance with the
GitHub CLI:

```sh
version=1.2.3
gh attestation verify "terraform-provider-openai_${version}_linux_amd64.zip" \
  --repo openai/terraform-provider-openai \
  --signer-workflow openai/terraform-provider-openai/.github/workflows/release.yml \
  --source-ref "refs/tags/v${version}"
```

Set `version` to the downloaded provider version and choose the archive for the
appropriate operating system and architecture.
