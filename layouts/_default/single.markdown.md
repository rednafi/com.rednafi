---
title: {{ .Title }}
date: {{ .Date.Format "2006-01-02" }}
{{- with .Params.slug }}
slug: {{ . }}
{{- end }}
{{- with .Params.aliases }}
aliases:
{{- range . }}
    - {{ . }}
{{- end }}
{{- end }}
{{- with .Params.tags }}
tags:
{{- range . }}
    - {{ . }}
{{- end }}
{{- end }}
{{- with .Description }}
description: >-
    {{ . }}
{{- end }}
---

{{ .RawContent }}
