# Cluster Kubernetes gerenciado (EKS). A AWS cuida do "plano de controle"
# (API do Kubernetes, etcd); nós cuidamos só dos nós de trabalho.
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = local.name
  kubernetes_version = var.kubernetes_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # API do cluster acessível pela internet (protegida por IAM). Para fechar
  # totalmente, use false e acesse via VPN/bastion.
  endpoint_public_access = true

  # Quem cria o cluster recebe acesso de administrador (necessário no começo).
  enable_cluster_creator_admin_permissions = true

  addons = {
    coredns                = {}
    kube-proxy             = {}
    vpc-cni                = { before_compute = true }
    eks-pod-identity-agent = { before_compute = true }
  }

  eks_managed_node_groups = {
    default = {
      ami_type       = "AL2023_ARM_64_STANDARD"
      instance_types = var.node_instance_types
      min_size       = var.node_min_size
      max_size       = var.node_max_size
      desired_size   = var.node_min_size
    }
  }

  # O pipeline de deploy (GitHub Actions) só administra os namespaces do InnKeep.
  access_entries = {
    github_deploy = {
      principal_arn = aws_iam_role.github_deploy.arn
      policy_associations = {
        innkeep = {
          policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSAdminPolicy"
          access_scope = {
            type       = "namespace"
            namespaces = ["innkeep-${var.environment}"]
          }
        }
      }
    }
  }
}
