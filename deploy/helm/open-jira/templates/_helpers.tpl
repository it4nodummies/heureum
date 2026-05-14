{{- define "open-jira.fullname" -}}
{{- printf "open-jira" }}
{{- end }}

{{- define "open-jira.labels" -}}
app.kubernetes.io/name: open-jira
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "open-jira.selectorLabels" -}}
app.kubernetes.io/name: open-jira
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
