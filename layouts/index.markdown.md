{{- $articles := where site.RegularPages "Type" "in" site.Params.mainSections -}}
{{- $notes := where site.RegularPages "Section" site.Params.notesSection -}}
{{- $pages := union $articles $notes -}}
{{- $pages = $pages.ByDate.Reverse -}}
{{- $source := replaceRE `/index\.md$` "/" .Permalink -}}
# {{ site.Title }}

Source: {{ $source }}

{{ site.Params.description | plainify }}

## Pages

{{ range .Site.Menus.pages }}
- [{{ .Name }}]({{ .URL | absURL }})
{{ end }}

## Browse

{{ range .Site.Menus.browse }}
- [{{ .Name }}]({{ .URL | absURL }})
{{ end }}

## Recent posts

{{ range first 30 $pages }}
- {{ .Date.Format "2006-01-02" }} [{{ .Title }}]({{ replaceRE `/index\.md$` "/" .Permalink }}){{ if .Params.type_label }} · {{ .Params.type_label }}{{ else if eq .Section site.Params.notesSection }} · shard{{ else }} · article{{ end }}
{{ end }}
