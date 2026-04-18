{{- $source := replaceRE `/index\.md$` "/" .Permalink -}}
# {{ .Title }}

Source: {{ $source }}

This is a utility page for the browser search UI and is excluded from indexing.

Use these pages instead:

- [Archive]({{ "archive/" | absURL }})
- [Tags]({{ "tags/" | absURL }})
- [llms.txt]({{ "llms.txt" | absURL }})
- [Sitemap]({{ "sitemap.xml" | absURL }})
