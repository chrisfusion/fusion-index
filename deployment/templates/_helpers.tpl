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
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Database host.
*/}}
{{- define "fusion-index.dbHost" -}}
{{- .Values.postgresql.host -}}
{{- end }}

{{/*
Database port.
*/}}
{{- define "fusion-index.dbPort" -}}
{{- .Values.postgresql.port | default 5432 -}}
{{- end }}

{{/*
Database name.
*/}}
{{- define "fusion-index.dbName" -}}
{{- .Values.postgresql.database -}}
{{- end }}

{{/*
Database username.
*/}}
{{- define "fusion-index.dbUsername" -}}
{{- .Values.postgresql.username -}}
{{- end }}

{{/*
Secret name that contains the database password (key: "password").

Resolution order:
  1. existingSecret set    → user-provided secret
  2. no existingSecret     → chart-managed secret (<release>-db-secret)
*/}}
{{- define "fusion-index.dbSecretName" -}}
{{- if .Values.postgresql.existingSecret -}}
  {{- .Values.postgresql.existingSecret -}}
{{- else -}}
  {{- printf "%s-db-secret" .Release.Name -}}
{{- end -}}
{{- end }}

{{/*
Secret name holding the PostgreSQL admin/superuser credentials (key matching
postgresql.admin.existingSecretKey) used by the one-time create-database Job.
Not the same secret as dbSecretName above, which holds the app's own runtime
DB_PASSWORD.

Resolution order:
  1. postgresql.admin.existingSecret set → user-provided secret
  2. no existingSecret                   → chart-managed secret (<release>-postgresql-admin)
*/}}
{{- define "fusion-index.pgAdminSecretName" -}}
{{- if .Values.postgresql.admin.existingSecret -}}
  {{- .Values.postgresql.admin.existingSecret -}}
{{- else -}}
  {{- printf "%s-postgresql-admin" .Release.Name -}}
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
