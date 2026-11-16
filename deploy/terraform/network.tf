data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name = "innkeep-${var.environment}"
  # Três zonas de disponibilidade: a queda de um datacenter da AWS não derruba o sistema.
  azs = slice(data.aws_availability_zones.available.names, 0, 3)
}

# Rede (VPC) com três camadas de sub-redes:
#   - públicas:  só o balanceador de carga (ALB) e os NAT Gateways;
#   - privadas:  os nós do Kubernetes (sem IP público);
#   - de banco:  RDS, MSK e ElastiCache, acessíveis apenas de dentro da VPC.
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.0"

  name = local.name
  cidr = var.vpc_cidr
  azs  = local.azs

  public_subnets   = [for index, _ in local.azs : cidrsubnet(var.vpc_cidr, 8, index)]
  private_subnets  = [for index, _ in local.azs : cidrsubnet(var.vpc_cidr, 4, index + 1)]
  database_subnets = [for index, _ in local.azs : cidrsubnet(var.vpc_cidr, 8, index + 100)]

  create_database_subnet_group = true
  enable_dns_hostnames         = true

  # Os nós privados saem para a internet (Booking.com, Docker Hub...) pelo NAT.
  # Em staging um único NAT economiza; em produção, um por zona.
  enable_nat_gateway = true
  single_nat_gateway = var.environment != "production"

  # Rótulos que o AWS Load Balancer Controller usa para descobrir as sub-redes.
  public_subnet_tags  = { "kubernetes.io/role/elb" = 1 }
  private_subnet_tags = { "kubernetes.io/role/internal-elb" = 1 }
}

# Grupo de segurança dos dados: aceita conexões SOMENTE dos nós do EKS.
resource "aws_security_group" "data" {
  name        = "${local.name}-data"
  description = "PostgreSQL, Kafka e Redis acessiveis apenas pelo cluster"
  vpc_id      = module.vpc.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "data_from_cluster" {
  for_each = {
    postgres = 5432
    redis    = 6379
    kafka    = 9096 # MSK com TLS + SASL/SCRAM
  }
  security_group_id            = aws_security_group.data.id
  referenced_security_group_id = module.eks.node_security_group_id
  from_port                    = each.value
  to_port                      = each.value
  ip_protocol                  = "tcp"
  description                  = "${each.key} a partir dos nos do EKS"
}
