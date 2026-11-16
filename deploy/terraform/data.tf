# =============================================================================
# Dados: PostgreSQL (RDS), Kafka (MSK) e Redis (ElastiCache) — todos serviços
# GERENCIADOS. A AWS cuida de backup, atualização, réplicas e failover; nós
# não operamos bancos de dados em contêineres em produção.
# =============================================================================

# ---- PostgreSQL ------------------------------------------------------------
# Uma instância, um banco por microserviço (criados pelo script de bootstrap).
resource "aws_db_parameter_group" "postgres" {
  name   = "${local.name}-postgres"
  family = "postgres${split(".", var.postgres_engine_version)[0]}"

  # Recusa conexões sem TLS.
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
  # Registra consultas lentas (> 500 ms) para investigação.
  parameter {
    name  = "log_min_duration_statement"
    value = "500"
  }
}

resource "aws_db_instance" "postgres" {
  identifier     = local.name
  engine         = "postgres"
  engine_version = var.postgres_engine_version
  instance_class = var.db_instance_class

  allocated_storage     = 20
  max_allocated_storage = 200 # cresce sozinho quando precisa
  storage_type          = "gp3"
  storage_encrypted     = true

  username = "innkeep_admin"
  # A senha do administrador é gerada e guardada pela própria AWS no Secrets
  # Manager — ela nunca aparece no Terraform nem no estado.
  manage_master_user_password = true

  db_subnet_group_name   = module.vpc.database_subnet_group_name
  vpc_security_group_ids = [aws_security_group.data.id]
  parameter_group_name   = aws_db_parameter_group.postgres.name
  publicly_accessible    = false

  multi_az                     = var.environment == "production" # réplica síncrona em outra zona
  backup_retention_period      = var.environment == "production" ? 14 : 3
  deletion_protection          = var.environment == "production"
  skip_final_snapshot          = var.environment != "production"
  final_snapshot_identifier    = var.environment == "production" ? "${local.name}-final" : null
  performance_insights_enabled = true
  auto_minor_version_upgrade   = true
}

# ---- Kafka (Amazon MSK) ----------------------------------------------------
resource "aws_msk_configuration" "kafka" {
  name           = local.name
  kafka_versions = [var.kafka_version]

  # Mesmas regras do ambiente local, mais robustas:
  #  - tópicos só são criados de propósito (nunca por acidente);
  #  - cada mensagem fica em 3 brokers e só é confirmada com 2 cópias gravadas.
  server_properties = <<-PROPERTIES
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    num.partitions=3
    log.retention.hours=168
  PROPERTIES
}

resource "aws_msk_cluster" "kafka" {
  cluster_name           = local.name
  kafka_version          = var.kafka_version
  number_of_broker_nodes = 3 # um por zona de disponibilidade

  broker_node_group_info {
    instance_type   = var.kafka_instance_type
    client_subnets  = module.vpc.database_subnets
    security_groups = [aws_security_group.data.id]
    storage_info {
      ebs_storage_info {
        volume_size = 50
      }
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.kafka.arn
    revision = aws_msk_configuration.kafka.latest_revision
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  client_authentication {
    sasl {
      scram = true
    }
  }
}

# Credenciais SASL/SCRAM dos serviços. O MSK exige que o segredo comece com
# "AmazonMSK_" e seja cifrado com uma chave KMS própria (não a padrão).
resource "aws_kms_key" "msk" {
  description         = "${local.name} - credenciais SCRAM do MSK"
  enable_key_rotation = true
}

resource "random_password" "kafka" {
  length  = 32
  special = false
}

resource "aws_secretsmanager_secret" "kafka" {
  name       = "AmazonMSK_${local.name}"
  kms_key_id = aws_kms_key.msk.key_id
}

resource "aws_secretsmanager_secret_version" "kafka" {
  secret_id     = aws_secretsmanager_secret.kafka.id
  secret_string = jsonencode({ username = "innkeep", password = random_password.kafka.result })
}

resource "aws_msk_scram_secret_association" "kafka" {
  cluster_arn     = aws_msk_cluster.kafka.arn
  secret_arn_list = [aws_secretsmanager_secret.kafka.arn]
  depends_on      = [aws_secretsmanager_secret_version.kafka]
}

# ---- Redis (ElastiCache) ---------------------------------------------------
resource "random_password" "redis" {
  length  = 40
  special = false
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = local.name
  subnet_ids = module.vpc.database_subnets
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = local.name
  description          = "InnKeep - cache, rate limit e idempotencia"
  engine               = "redis"
  node_type            = var.redis_node_type
  port                 = 6379

  # Primário + réplica em outra zona, com failover automático.
  num_cache_clusters         = var.environment == "production" ? 2 : 1
  automatic_failover_enabled = var.environment == "production"
  multi_az_enabled           = var.environment == "production"

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.data.id]

  # TLS e senha: os serviços usam rediss://:<senha>@<endpoint>:6379/0
  transit_encryption_enabled = true
  at_rest_encryption_enabled = true
  auth_token                 = random_password.redis.result
}
