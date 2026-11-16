# =============================================================================
# Entrega: registro de imagens (ECR), segredos da aplicação e o papel IAM que o
# GitHub Actions assume via OIDC para publicar e implantar.
# =============================================================================

# ---- ECR: uma imagem por serviço -------------------------------------------
resource "aws_ecr_repository" "service" {
  for_each = toset(var.services)

  name = "innkeep/${each.key}"
  # Tags imutáveis: a tag <sha> sempre aponta para a mesma imagem.
  image_tag_mutability = "IMMUTABLE"
  # Varredura de vulnerabilidades a cada push.
  image_scanning_configuration {
    scan_on_push = true
  }
  encryption_configuration {
    encryption_type = "AES256"
  }
}

resource "aws_ecr_lifecycle_policy" "service" {
  for_each   = aws_ecr_repository.service
  repository = each.value.name
  # Mantém as 30 imagens mais recentes; o resto é apagado (custo de armazenamento).
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Manter as 30 imagens mais recentes"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 30 }
      action       = { type = "expire" }
    }]
  })
}

# ---- Segredos da aplicação -------------------------------------------------
# O Terraform cria apenas o "cofre" vazio. Os valores (URLs dos bancos, chaves
# de criptografia, chave JWT) são gravados pelo script
# scripts/bootstrap-secrets.sh — assim eles não ficam no estado do Terraform.
# O External Secrets Operator lê este segredo e o entrega aos pods.
resource "aws_secretsmanager_secret" "app" {
  name                    = "innkeep/${var.environment}"
  description             = "Variaveis sensiveis do InnKeep (${var.environment})"
  recovery_window_in_days = var.environment == "production" ? 30 : 0
}

# ---- GitHub Actions via OIDC -----------------------------------------------
# O GitHub prova a identidade do workflow com um token assinado; a AWS confere
# e entrega credenciais temporárias. Nenhuma chave de acesso permanente existe.
resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_policy_document" "github_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    # Só workflows da branch main ou dos environments deste repositório.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        "repo:${var.github_repository}:ref:refs/heads/main",
        "repo:${var.github_repository}:environment:${var.environment}",
      ]
    }
  }
}

resource "aws_iam_role" "github_deploy" {
  name               = "${local.name}-github-deploy"
  assume_role_policy = data.aws_iam_policy_document.github_trust.json
}

data "aws_iam_policy_document" "github_deploy" {
  statement {
    sid       = "EcrLogin"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid = "EcrPush"
    actions = [
      "ecr:BatchCheckLayerAvailability", "ecr:BatchGetImage", "ecr:CompleteLayerUpload",
      "ecr:GetDownloadUrlForLayer", "ecr:InitiateLayerUpload", "ecr:PutImage", "ecr:UploadLayerPart",
    ]
    resources = [for repository in aws_ecr_repository.service : repository.arn]
  }
  statement {
    sid       = "EksDescribe"
    actions   = ["eks:DescribeCluster"]
    resources = [module.eks.cluster_arn]
  }
}

resource "aws_iam_role_policy" "github_deploy" {
  name   = "deploy"
  role   = aws_iam_role.github_deploy.id
  policy = data.aws_iam_policy_document.github_deploy.json
}
