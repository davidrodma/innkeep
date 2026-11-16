{{/* Nome completo de um componente: innkeep-<nome>. */}}
{{- define "innkeep.fullname" -}}
{{- printf "%s-%s" .root.Release.Name .name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Rótulos padrão do Kubernetes (app.kubernetes.io/*). */}}
{{- define "innkeep.labels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/part-of: innkeep
app.kubernetes.io/version: {{ .root.Values.global.image.tag | default .root.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end -}}

{{- define "innkeep.selector" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- end -}}

{{/* Imagem de um componente: <registry>/<nome>:<tag>. */}}
{{- define "innkeep.image" -}}
{{- printf "%s/%s:%s" .root.Values.global.image.registry .name (.root.Values.global.image.tag | default .root.Chart.AppVersion) -}}
{{- end -}}

{{/*
Contexto de segurança dos contêineres: usuário sem privilégios, sistema de
arquivos somente leitura, nenhuma capability do Linux. Um invasor que explore
uma falha no serviço encontra um ambiente onde quase nada é possível.
*/}}
{{- define "innkeep.containerSecurity" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
runAsNonRoot: true
capabilities:
  drop: ["ALL"]
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/* Variáveis de ambiente comuns a todos os serviços Go. */}}
{{- define "innkeep.commonEnv" -}}
- name: SERVICE_NAME
  value: {{ .name | quote }}
- name: APP_ENV
  value: {{ .root.Values.global.environment | quote }}
- name: APP_VERSION
  value: {{ .root.Values.global.image.tag | quote }}
- name: APP_TIMEZONE
  value: {{ .root.Values.global.timezone | quote }}
- name: LOG_LEVEL
  value: {{ .root.Values.global.logLevel | quote }}
- name: HTTP_ADDR
  value: ":8080"
- name: AUTH_JWKS_URL
  value: {{ printf "http://%s-identity:8080/.well-known/jwks.json" .root.Release.Name | quote }}
- name: AUTH_ISSUER
  value: {{ .root.Values.global.auth.issuer | quote }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ .root.Values.global.otlpEndpoint | quote }}
- name: PUBLIC_WEB_URL
  value: {{ .root.Values.global.publicWebUrl | quote }}
- name: PUBLIC_API_URL
  value: {{ .root.Values.global.publicApiUrl | quote }}
# Em produção as migrações rodam no Job de pré-deploy, não no início do pod.
- name: DATABASE_AUTO_MIGRATE
  value: "false"
{{- end -}}

{{/* Variáveis de Kafka (Amazon MSK com TLS + SASL/SCRAM). */}}
{{- define "innkeep.kafkaEnv" -}}
- name: KAFKA_BROKERS
  value: {{ .root.Values.global.kafka.brokers | quote }}
- name: KAFKA_TLS
  value: {{ .root.Values.global.kafka.tls | quote }}
- name: KAFKA_SASL_MECHANISM
  value: {{ .root.Values.global.kafka.saslMechanism | quote }}
- name: KAFKA_ENSURE_TOPICS
  value: {{ .root.Values.global.kafka.ensureTopics | quote }}
{{- end -}}

{{/*
Variáveis vindas do Secret. DATABASE_URL_<SERVIÇO> vira DATABASE_URL dentro do
contêiner: cada serviço conhece apenas a URL do SEU banco.
*/}}
{{- define "innkeep.secretEnv" -}}
{{- range .keys }}
- name: {{ hasPrefix "DATABASE_URL_" . | ternary "DATABASE_URL" . }}
  valueFrom:
    secretKeyRef:
      name: {{ $.root.Values.secrets.name }}
      key: {{ . }}
{{- end }}
{{- end -}}
