# syntax=docker/dockerfile:1
#
# Imagem do front-end (Next.js), em três etapas:
#   1. deps    — instala as dependências (camada reaproveitada enquanto o
#                package-lock.json não mudar);
#   2. build   — compila o site (`next build`, saída "standalone");
#   3. runtime — imagem final só com o servidor mínimo e os arquivos estáticos.

ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# `npm ci` instala exatamente o que está no package-lock (build reproduzível).
RUN --mount=type=cache,target=/root/.npm npm ci

FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:${NODE_VERSION}-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

# A imagem oficial do Node já traz o usuário sem privilégios "node".
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public

USER node
EXPOSE 3000
CMD ["node", "server.js"]
