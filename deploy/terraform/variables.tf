variable "region" {
  description = "Região da AWS. sa-east-1 (São Paulo) mantém os dados no Brasil e a latência baixa."
  type        = string
  default     = "sa-east-1"
}

variable "environment" {
  description = "Nome do ambiente: staging ou production."
  type        = string
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "Use staging ou production."
  }
}

variable "vpc_cidr" {
  description = "Faixa de IPs da VPC."
  type        = string
  default     = "10.40.0.0/16"
}

variable "kubernetes_version" {
  description = "Versão do Kubernetes no EKS."
  type        = string
  default     = "1.34"
}

variable "node_instance_types" {
  description = "Tipos de instância dos nós do EKS (Graviton/ARM é ~20% mais barato)."
  type        = list(string)
  default     = ["t4g.large"]
}

variable "node_min_size" {
  type    = number
  default = 2
}

variable "node_max_size" {
  type    = number
  default = 6
}

variable "postgres_engine_version" {
  description = "Versão do PostgreSQL no RDS (o esquema é compatível com 16 ou superior)."
  type        = string
  default     = "17"
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.medium"
}

variable "kafka_instance_type" {
  type    = string
  default = "kafka.t3.small"
}

variable "kafka_version" {
  description = "Versão do Apache Kafka no Amazon MSK."
  type        = string
  default     = "3.9.x"
}

variable "redis_node_type" {
  type    = string
  default = "cache.t4g.small"
}

variable "github_repository" {
  description = "Repositório autorizado a fazer deploy via OIDC (formato dono/nome)."
  type        = string
  default     = "davidrodma/innkeep"
}

variable "services" {
  description = "Imagens publicadas no ECR."
  type        = list(string)
  default     = ["gateway", "identity", "property", "inventory", "reservation", "billing", "guest", "channel", "notification", "web"]
}
