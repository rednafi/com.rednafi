{{- $desc := .Description | default .Summary | plainify -}}
{{- $body := .RawContent -}}
{{- $source := replaceRE `/index\.md$` "/" .Permalink -}}
{{- $body = replaceRE `\{\{<\s*mermaid\s*>\}\}` "```mermaid" $body -}}
{{- $body = replaceRE `\{\{<\s*/mermaid\s*>\}\}|\{\{</\s*mermaid\s*>\}\}` "```" $body -}}
{{- $body = replaceRE `\{\{<\s*youtube\s+([A-Za-z0-9_-]+)\s*>\}\}` `[YouTube video](https://www.youtube.com/watch?v=$1)` $body -}}
{{- $body = strings.TrimSpace $body -}}
# {{ .Title }}

Source: {{ $source }}
Published: {{ .Date.Format "2006-01-02" }}
Updated: {{ .Lastmod.Format "2006-01-02" }}
{{- with .Section }}
Section: {{ . }}
{{- end }}
{{- with .Params.tags }}
Tags: {{ delimit . ", " }}
{{- end }}
{{- with $desc }}
Summary: {{ . }}
{{- end }}

---

{{- if .Params.robotsNoIndex }}
This page is excluded from indexing and is kept only as a utility page.
{{- else if $body }}
{{ $body }}
{{- else }}
{{ .Plain }}
{{- end }}
