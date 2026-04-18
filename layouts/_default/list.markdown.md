{{- $desc := .Description | default .Summary | plainify | default site.Params.description -}}
{{- $source := replaceRE `/index\.md$` "/" .Permalink -}}
# {{ .Title }}

Source: {{ $source }}

{{- with $desc }}
{{ . }}
{{- end }}

{{- if eq .Kind "taxonomy" }}
## Topics

{{ range .Pages }}
- [{{ .Title }}]({{ replaceRE `/index\.md$` "/" .Permalink }}) ({{ len .Pages }})
{{ end }}
{{- else }}
## Entries

{{ range .RegularPages }}
- {{ .Date.Format "2006-01-02" }} [{{ .Title }}]({{ replaceRE `/index\.md$` "/" .Permalink }})
{{ end }}
{{- end }}
