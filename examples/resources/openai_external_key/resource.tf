variable "openai_organization_id" {
  type        = string
  description = "OpenAI organization ID used as the AWS STS external ID."
}

variable "aws_kms_administrator_arns" {
  type        = set(string)
  description = "Existing AWS principal ARNs allowed to administer the KMS key."
}

data "aws_iam_policy_document" "openai_ekm_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::790389265272:role/EnterpriseKeyManagement"]
    }

    condition {
      test     = "StringEquals"
      variable = "sts:ExternalId"
      values   = [var.openai_organization_id]
    }
  }
}

resource "aws_iam_role" "openai_ekm" {
  name               = "openai-ekm"
  assume_role_policy = data.aws_iam_policy_document.openai_ekm_trust.json
}

data "aws_iam_policy_document" "openai_ekm_key" {
  statement {
    sid = "AllowKeyAdministration"
    actions = [
      "kms:CancelKeyDeletion",
      "kms:Create*",
      "kms:Delete*",
      "kms:Describe*",
      "kms:Disable*",
      "kms:Enable*",
      "kms:Get*",
      "kms:List*",
      "kms:Put*",
      "kms:Revoke*",
      "kms:RotateKeyOnDemand",
      "kms:ScheduleKeyDeletion",
      "kms:TagResource",
      "kms:UntagResource",
      "kms:Update*",
    ]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = tolist(var.aws_kms_administrator_arns)
    }
  }

  statement {
    sid = "AllowOpenAIEKMUse"
    actions = [
      "kms:Decrypt",
      "kms:Encrypt",
    ]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.openai_ekm.arn]
    }
  }
}

resource "aws_kms_key" "openai" {
  description             = "OpenAI external key management"
  deletion_window_in_days = 30
  enable_key_rotation     = true
  policy                  = data.aws_iam_policy_document.openai_ekm_key.json
}

data "aws_iam_policy_document" "openai_ekm" {
  statement {
    actions = [
      "kms:Decrypt",
      "kms:Encrypt",
    ]
    resources = [aws_kms_key.openai.arn]
  }
}

resource "aws_iam_role_policy" "openai_ekm" {
  name   = "openai-ekm"
  role   = aws_iam_role.openai_ekm.id
  policy = data.aws_iam_policy_document.openai_ekm.json
}

resource "openai_external_key" "example" {
  name        = "production-ekm"
  kms_arn     = aws_kms_key.openai.arn
  role_arn    = aws_iam_role.openai_ekm.arn
  external_id = var.openai_organization_id

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [aws_iam_role_policy.openai_ekm]
}

resource "openai_project" "encrypted" {
  name            = "encrypted-project"
  external_key_id = openai_external_key.example.external_key_id
}
