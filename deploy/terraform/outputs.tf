output "cluster_name" {
  description = "Nome do cluster EKS (variável EKS_CLUSTER_NAME do GitHub)."
  value       = module.eks.cluster_name
}

output "github_deploy_role_arn" {
  description = "IAM Role do pipeline (variável AWS_DEPLOY_ROLE_ARN do GitHub)."
  value       = aws_iam_role.github_deploy.arn
}

output "ecr_repositories" {
  description = "Endereço de cada repositório de imagens."
  value       = { for name, repository in aws_ecr_repository.service : name => repository.repository_url }
}

output "postgres_endpoint" {
  description = "Endereço do PostgreSQL (host:porta)."
  value       = aws_db_instance.postgres.endpoint
}

output "postgres_master_secret_arn" {
  description = "Segredo (gerado pela AWS) com a senha do administrador do PostgreSQL."
  value       = aws_db_instance.postgres.master_user_secret[0].secret_arn
}

output "msk_bootstrap_brokers_sasl_scram" {
  description = "Brokers do Kafka (KAFKA_BROKERS) com TLS + SASL/SCRAM."
  value       = aws_msk_cluster.kafka.bootstrap_brokers_sasl_scram
}

output "msk_credentials_secret_arn" {
  description = "Segredo com usuário e senha SCRAM do Kafka."
  value       = aws_secretsmanager_secret.kafka.arn
}

output "redis_primary_endpoint" {
  description = "Endereço primário do Redis."
  value       = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "redis_auth_token" {
  description = "Senha do Redis (use em REDIS_URL). Sensível: não aparece no terminal sem -raw."
  value       = random_password.redis.result
  sensitive   = true
}

output "app_secret_name" {
  description = "Nome do segredo da aplicação no Secrets Manager."
  value       = aws_secretsmanager_secret.app.name
}
