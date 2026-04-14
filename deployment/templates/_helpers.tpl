{{/*
Backend Kubernetes Service name.
Must be unique within the namespace. Defaults to "fusion-index-backend".
*/}}
{{- define "fusion-index.backendServiceName" -}}
{{- .Values.backend.serviceName | default "fusion-index-backend" }}
{{- end }}

{{/*
Standard Helm labels applied to every resource.
*/}}
{{- define "fusion-index.labels" -}}
app.kubernetes.io/name: fusion-index
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Database host — Bitnami subchart FQDN or external host.
*/}}
{{- define "fusion-index.dbHost" -}}
{{- if .Values.postgresql.enabled -}}
{{- printf "%s-postgresql.%s.svc.cluster.local" .Release.Name .Values.namespace -}}
{{- else -}}
{{- .Values.postgresql.external.host -}}
{{- end -}}
{{- end }}

{{/*
Database port.
*/}}
{{- define "fusion-index.dbPort" -}}
{{- if .Values.postgresql.enabled -}}
5432
{{- else -}}
{{- .Values.postgresql.external.port | default 5432 -}}
{{- end -}}
{{- end }}

{{/*
Database name.
*/}}
{{- define "fusion-index.dbName" -}}
{{- if .Values.postgresql.enabled -}}
{{- .Values.postgresql.auth.database -}}
{{- else -}}
{{- .Values.postgresql.external.database -}}
{{- end -}}
{{- end }}

{{/*
Database username.
*/}}
{{- define "fusion-index.dbUsername" -}}
{{- if .Values.postgresql.enabled -}}
{{- .Values.postgresql.auth.username -}}
{{- else -}}
{{- .Values.postgresql.external.username -}}
{{- end -}}
{{- end }}

{{/*
Secret name that contains the database password (key: "password").

Resolution order:
  1. Bundled postgresql + existingSecret set  → user-provided secret
  2. Bundled postgresql, no existingSecret    → Bitnami auto-generated secret (<release>-postgresql)
  3. External DB + existingSecret set         → user-provided secret
  4. External DB, no existingSecret           → chart-managed secret (<release>-db-secret)
*/}}
{{- define "fusion-index.dbSecretName" -}}
{{- if .Values.postgresql.enabled -}}
  {{- if .Values.postgresql.auth.existingSecret -}}
    {{- .Values.postgresql.auth.existingSecret -}}
  {{- else -}}
    {{- printf "%s-postgresql" .Release.Name -}}
  {{- end -}}
{{- else -}}
  {{- if .Values.postgresql.external.existingSecret -}}
    {{- .Values.postgresql.external.existingSecret -}}
  {{- else -}}
    {{- printf "%s-db-secret" .Release.Name -}}
  {{- end -}}
{{- end -}}
{{- end }}

{{/*
Secret name that contains S3 credentials.

Resolution order:
  1. s3.existingSecret set         → user-provided secret
  2. credentialsType = default     → no secret (IRSA / Workload Identity)
  3. credentialsType = static      → chart-managed secret (<release>-s3-secret)
*/}}
{{- define "fusion-index.s3SecretName" -}}
{{- if .Values.s3.existingSecret -}}
  {{- .Values.s3.existingSecret -}}
{{- else -}}
  {{- printf "%s-s3-secret" .Release.Name -}}
{{- end -}}
{{- end }}
