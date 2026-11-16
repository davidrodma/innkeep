#!/usr/bin/env bash
# Valida o chart Helm e o Terraform sem instalar nenhuma das ferramentas:
# ambas rodam em contêineres Docker oficiais.
#
#   ./scripts/validate-infra.sh
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELM_IMAGE=alpine/helm:3.19.0
TERRAFORM_IMAGE=hashicorp/terraform:1.13

echo ">>> helm lint + template"
docker run --rm -v "${ROOT}/deploy/helm":/charts -w /charts "$HELM_IMAGE" lint innkeep \
  --set global.image.registry=example.com/innkeep --set global.image.tag=test
docker run --rm -v "${ROOT}/deploy/helm":/charts -w /charts "$HELM_IMAGE" template innkeep innkeep \
  -f innkeep/values-production.yaml \
  --set global.image.registry=example.com/innkeep --set global.image.tag=test > /tmp/innkeep-rendered.yaml
echo "manifestos renderizados: $(grep -c '^kind:' /tmp/innkeep-rendered.yaml) recursos"
grep '^kind:' /tmp/innkeep-rendered.yaml | sort | uniq -c

echo ">>> terraform fmt"
docker run --rm -v "${ROOT}/deploy/terraform":/tf -w /tf "$TERRAFORM_IMAGE" fmt -check -recursive

echo ">>> terraform init + validate"
# .terraform fica em um volume do Docker (não no seu disco), e o git de dentro
# do contêiner precisa confiar no diretório montado para baixar os módulos.
docker run --rm \
  -v "${ROOT}/deploy/terraform":/tf \
  -v innkeep-tf-dotterraform:/tf/.terraform \
  -v innkeep-tf-plugins:/plugins \
  -e TF_PLUGIN_CACHE_DIR=/plugins \
  -w /tf --entrypoint sh "$TERRAFORM_IMAGE" -c \
  'git config --global --add safe.directory "*" && terraform init -backend=false -input=false > /tmp/init.log 2>&1 || { tail -30 /tmp/init.log; exit 1; }; terraform validate'
