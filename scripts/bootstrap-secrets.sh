#!/usr/bin/env bash
# =============================================================================
# Prepara os segredos de um ambiente na AWS (rodar UMA vez por ambiente, depois
# do `terraform apply`). Ele:
#
#   1. cria, no RDS, um usuário e um banco para cada microserviço ("banco por
#      serviço"), cada um com a sua própria senha;
#   2. gera as chaves de criptografia e a chave de assinatura dos JWT;
#   3. grava tudo no AWS Secrets Manager (segredo innkeep/<ambiente>), de onde
#      o External Secrets Operator entrega aos pods.
#
# Nenhum valor é impresso na tela nem salvo em disco. Rode de uma máquina com
# acesso à rede do RDS (ex.: um pod temporário no cluster ou via bastion/VPN).
#
# Uso: ./scripts/bootstrap-secrets.sh production
# Requisitos: aws (CLI v2), psql, jq, openssl, terraform (com o estado do ambiente).
# =============================================================================
set -euo pipefail

ENVIRONMENT="${1:?informe o ambiente: staging ou production}"
cd "$(dirname "$0")/../deploy/terraform"

output() { terraform output -raw "$1"; }

REGION="${AWS_REGION:-sa-east-1}"
PG_ENDPOINT="$(output postgres_endpoint)"
PG_HOST="${PG_ENDPOINT%%:*}"
MASTER="$(aws secretsmanager get-secret-value --region "$REGION" --secret-id "$(output postgres_master_secret_arn)" --query SecretString --output text)"
KAFKA="$(aws secretsmanager get-secret-value --region "$REGION" --secret-id "$(output msk_credentials_secret_arn)" --query SecretString --output text)"
REDIS_HOST="$(output redis_primary_endpoint)"
REDIS_TOKEN="$(output redis_auth_token)"

export PGHOST="$PG_HOST" PGPORT=5432 PGDATABASE=postgres PGSSLMODE=require
export PGUSER="$(jq -r .username <<<"$MASTER")" PGPASSWORD="$(jq -r .password <<<"$MASTER")"

secret='{}'
put() { secret="$(jq --arg k "$1" --arg v "$2" '. + {($k): $v}' <<<"$secret")"; }

for service in identity property inventory reservation billing guest channel; do
  password="$(openssl rand -hex 24)"
  # Cria (ou atualiza a senha de) um usuário dono de um banco exclusivo.
  psql -v ON_ERROR_STOP=1 -q <<SQL
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${service}') THEN
    CREATE ROLE ${service} LOGIN;
  END IF;
END \$\$;
ALTER ROLE ${service} WITH PASSWORD '${password}';
SQL
  psql -q -tc "SELECT 1 FROM pg_database WHERE datname = '${service}'" | grep -q 1 || \
    psql -v ON_ERROR_STOP=1 -q -c "CREATE DATABASE ${service} OWNER ${service}"
  psql -v ON_ERROR_STOP=1 -q -c "REVOKE ALL ON DATABASE ${service} FROM PUBLIC"
  put "DATABASE_URL_${service^^}" "postgres://${service}:${password}@${PG_ENDPOINT}/${service}?sslmode=require"
  echo "banco '${service}' pronto"
done

put REDIS_URL "rediss://:${REDIS_TOKEN}@${REDIS_HOST}:6379/0"
put KAFKA_USERNAME "$(jq -r .username <<<"$KAFKA")"
put KAFKA_PASSWORD "$(jq -r .password <<<"$KAFKA")"
put PII_ENCRYPTION_KEY "$(openssl rand -base64 32)"
put CHANNEL_ENCRYPTION_KEY "$(openssl rand -base64 32)"
# Semente Ed25519 de 32 bytes, em base64 (o mesmo formato de `identity keygen`).
put JWT_PRIVATE_KEY "$(openssl rand -base64 32)"
put BOOTSTRAP_ADMIN_EMAIL "${BOOTSTRAP_ADMIN_EMAIL:-admin@example.com}"
put BOOTSTRAP_ADMIN_PASSWORD "$(openssl rand -base64 18)"
# Credenciais SMTP do Amazon SES: crie no console do SES e preencha depois com
#   aws secretsmanager put-secret-value ...
put SMTP_USERNAME "${SMTP_USERNAME:-preencher}"
put SMTP_PASSWORD "${SMTP_PASSWORD:-preencher}"

aws secretsmanager put-secret-value --region "$REGION" \
  --secret-id "$(output app_secret_name)" --secret-string "$secret" > /dev/null

echo
echo "Segredo $(output app_secret_name) gravado."
echo "A senha inicial do administrador está no próprio segredo (chave BOOTSTRAP_ADMIN_PASSWORD)."
echo "ATENÇÃO: a chave PII_ENCRYPTION_KEY cifra os documentos dos hóspedes. Se for perdida,"
echo "os dados cifrados não podem ser recuperados — o Secrets Manager mantém o histórico de versões."
