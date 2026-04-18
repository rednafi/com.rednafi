{{- $source := replaceRE `/index\.md$` "/" .Permalink -}}
# {{ .Title }}

Source: {{ $source }}

Chronological archive of published writing.

{{ range (site.RegularPages).GroupByPublishDate "2006" }}
{{ if ne .Key "0001" }}
## {{ .Key }}

{{ range .Pages.GroupByDate "January" }}
### {{ .Key }}

{{ range .Pages }}
- {{ .Date.Format "2006-01-02" }} [{{ .Title }}]({{ replaceRE `/index\.md$` "/" .Permalink }})
{{ end }}

{{ end }}
{{ end }}
{{ end }}
