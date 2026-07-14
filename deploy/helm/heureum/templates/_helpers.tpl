{{- define "heureum.fullname" -}}
{{- printf "heureum" }}
{{- end }}

{{- define "heureum.labels" -}}
app.kubernetes.io/name: heureum
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "heureum.selectorLabels" -}}
app.kubernetes.io/name: heureum
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
