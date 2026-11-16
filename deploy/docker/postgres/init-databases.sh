#!/bin/bash
# Cria um banco e um usuário por microserviço.
#
# Este script roda uma única vez, na primeira inicialização do contêiner do
# PostgreSQL (convenção da imagem oficial: tudo em /docker-entrypoint-initdb.d).
#
# "Banco por serviço" é uma regra da arquitetura de microserviços: nenhum
# serviço lê ou escreve nas tabelas de outro. Aqui todos os bancos vivem na
# MESMA instância do PostgreSQL (mais barato e simples de operar), mas cada
# serviço só tem credenciais para o seu — o isolamento é lógico. Separar em
# instâncias diferentes depois é só mudar a URL de conexão.
set -euo pipefail

for service in identity property inventory reservation billing guest channel; do
  password_var="POSTGRES_${service^^}_PASSWORD"
  password="${!password_var:-${POSTGRES_SERVICE_PASSWORD:?defina POSTGRES_SERVICE_PASSWORD}}"

  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-SQL
    CREATE USER ${service} WITH PASSWORD '${password}';
    CREATE DATABASE ${service} OWNER ${service};
    -- Fecha o banco para os demais usuários: só o dono se conecta.
    REVOKE ALL ON DATABASE ${service} FROM PUBLIC;
SQL
  echo "banco '${service}' criado"
done
