SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
MAKEFLAGS += --silent

.PHONY: init format dev run upload-post-image img-upload

BREW_PACKAGES := go hugo node oxipng

PAGEFIND_VERSION ?= 1.5.2
PRETTIER_VERSION ?= 3.9.3
WRANGLER_VERSION ?= 4.83.0

PAGEFIND := npx -y pagefind@$(PAGEFIND_VERSION)
PRETTIER := npx -y prettier@$(PRETTIER_VERSION)
WRANGLER := npx -y wrangler@$(WRANGLER_VERSION)

init:
	if command -v brew >/dev/null 2>&1; then \
		for pkg in $(BREW_PACKAGES); do \
			brew list "$$pkg" >/dev/null 2>&1 || brew install "$$pkg"; \
		done; \
	else \
		for tool in go hugo node npm oxipng; do \
			command -v "$$tool" >/dev/null 2>&1 || { echo "$$tool is required"; exit 1; }; \
		done; \
	fi
	go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium

format:
	go run ./scripts/lintcodeblocks
	go run ./scripts/frontmatter
	$(PRETTIER) --write "content/**/*.md"

dev:
	go run ./scripts/frontmatter
	hugo --environment production --minify --gc --cleanDestinationDir
	$(PAGEFIND)
	rm -f \
		public/pagefind/pagefind-component-ui.css \
		public/pagefind/pagefind-component-ui.js \
		public/pagefind/pagefind-highlight.js \
		public/pagefind/pagefind-modular-ui.css \
		public/pagefind/pagefind-modular-ui.js \
		public/pagefind/wasm.unknown.pagefind
	hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache

run: dev

img-upload upload-post-image:
	@if [ -z "$(post)" ] || [ -z "$(file)" ] || [ -z "$(name)" ]; then \
		echo "Usage: make img-upload post=<content.md> file=<image> name=<kebab-name>"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make img-upload post=content/go/request_coalescing.md file=/tmp/diagram.png name=singleflight-flow"; \
		exit 1; \
	fi
	go run ./scripts/media --post "$(post)" --file "$(file)" --name "$(name)" --wrangler "$(WRANGLER)"
