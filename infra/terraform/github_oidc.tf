# GitHub Actions OIDC provider + deploy role for keyless CI. This replaces the
# long-lived AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY currently used by the
# deploy workflow (IA-5 / AC-6: no static credentials).
#
# Created ONLY when var.github_org is set. Leave it empty to skip OIDC entirely
# (e.g. you deploy manually with your own credentials, or have no GitHub repo
# wired up yet). The repo owner may be a personal username or an org.
locals {
  enable_github_oidc = var.github_org != ""
}

resource "aws_iam_openid_connect_provider" "github" {
  count          = local.enable_github_oidc ? 1 : 0
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]

  # GitHub's OIDC root CA thumbprint. AWS validates the JWT against its trusted
  # CA store, but the provider resource still requires a thumbprint value.
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]

  tags = {
    Name = "${var.name_prefix}-github-oidc"
  }
}

data "aws_iam_policy_document" "github_trust" {
  count = local.enable_github_oidc ? 1 : 0

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github[0].arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # Only workflow runs on the named repo + branch may assume the role.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_org}/${var.github_repo}:ref:refs/heads/${var.github_deploy_branch}"]
    }
  }
}

resource "aws_iam_role" "github_deploy" {
  count              = local.enable_github_oidc ? 1 : 0
  name               = "${var.name_prefix}-github-deploy"
  assume_role_policy = data.aws_iam_policy_document.github_trust[0].json

  tags = {
    Name = "${var.name_prefix}-github-deploy"
  }
}

# NOTE: deploy permissions (ECR push, ECS register/update task definitions and
# services) are attached in WS2 once those resources exist. The role is created
# here with trust only, so it is least-privilege by default.
