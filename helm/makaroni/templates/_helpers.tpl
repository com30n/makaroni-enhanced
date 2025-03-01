{{- define "makaroni.fullname" -}}
{{- printf "%s-%s" .Release.Name "makaroni" -}}
{{- end -}}

{{- define "makaroni.name" -}}
makaroni
{{- end -}}