# Infraestrutura do InnKeep na AWS, descrita como código (Terraform).
#
# Por que infraestrutura como código? Tudo o que existe na conta da AWS está
# descrito aqui, revisado em pull request e versionado no Git. Recriar o
# ambiente do zero — ou criar um ambiente de staging idêntico — é um comando.
terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # O estado (o "inventário" do que o Terraform criou) fica em um bucket S3,
  # com trava nativa (use_lockfile) para duas pessoas não aplicarem ao mesmo
  # tempo. Os valores vêm de backend-<ambiente>.hcl:
  #   terraform init -backend-config=environments/backend-production.hcl
  backend "s3" {}
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "innkeep"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
