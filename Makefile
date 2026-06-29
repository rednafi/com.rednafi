# Make all the rules since we don't use make as a build tool
.PHONY: $(MAKECMDGOALS)

SHELL := /bin/bash -ex
MAKEFLAGS += --silent

BREW_PACKAGES := gh hugo node
PAGEFIND_VERSION := 1.5.2
PRETTIER_VERSION := 3.9.3
SEQUOIA_CLI_VERSION := 0.5.7
WRANGLER_VERSION := 4.83.0
PAGES_SIZE_BUDGET_BYTES := 900000000
PAGEFIND := npx -y pagefind@$(PAGEFIND_VERSION)
PRETTIER := npx -y prettier@$(PRETTIER_VERSION)
WRANGLER := npx -y wrangler@$(WRANGLER_VERSION)

init:
	for pkg in $(BREW_PACKAGES); do \
		brew list $$pkg &>/dev/null || brew install $$pkg; \
	done
	go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium
	uvx pre-commit install

format:
	go run ./scripts/lintcodeblocks
	git status --porcelain -u | awk '{print $$2}' | xargs -r uvx pre-commit run --files
	git status --porcelain -u | awk '{print $$2}' | grep '.md' | xargs -n 1 $(PRETTIER) --write

update:
	uvx pre-commit autoupdate -j 4

frontmatter:
	go run ./scripts/frontmatter

check-frontmatter:
	go run ./scripts/frontmatter --check

postcards:
	go run ./scripts/postcards

postcards-missing:
	go run ./scripts/postcards --missing-r2-assets

check-postcards:
	go run ./scripts/postcards --check

compress-postcards:
	WRANGLER="$(WRANGLER)" bash ./scripts/upload-postcards --compress-only

upload-postcards:
	WRANGLER="$(WRANGLER)" bash ./scripts/upload-postcards

check-media:
	go run ./scripts/media --check

migrate-legacy-media:
	go run ./scripts/media --migrate-legacy --wrangler "$(WRANGLER)"

check-pages-size:
	bytes=$$(du -sk public | awk '{print $$1 * 1024}'); \
	echo "public/ is $$bytes bytes; budget is $(PAGES_SIZE_BUDGET_BYTES) bytes"; \
	if [ "$$bytes" -gt "$(PAGES_SIZE_BUDGET_BYTES)" ]; then \
		echo "public/ exceeds the GitHub Pages artifact budget"; \
		exit 1; \
	fi

prune-pagefind-extras:
	# This site uses the default Pagefind UI on a single-language en index.
	# Keep the runtime files the search page loads and prune optional bundles.
	rm -f \
		public/pagefind/pagefind-component-ui.css \
		public/pagefind/pagefind-component-ui.js \
		public/pagefind/pagefind-highlight.js \
		public/pagefind/pagefind-modular-ui.css \
		public/pagefind/pagefind-modular-ui.js \
		public/pagefind/wasm.unknown.pagefind

dev:
	go run ./scripts/frontmatter
	hugo --environment production --minify --gc --cleanDestinationDir
	$(PAGEFIND)
	$(MAKE) prune-pagefind-extras
	hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache

build:
	go run ./scripts/frontmatter
	hugo --environment production --minify --gc --cleanDestinationDir
	$(PAGEFIND)
	$(MAKE) prune-pagefind-extras
	$(MAKE) check-pages-size

test: build
	$(MAKE) check-frontmatter
	$(MAKE) check-postcards
	$(MAKE) check-media
	go test -v -count=1 ./...

upload-post-image:
	@if [ -z "$(post)" ] || [ -z "$(file)" ] || [ -z "$(name)" ]; then \
		echo "Usage: make upload-post-image post=<content.md> file=<image> name=<kebab-name>"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make upload-post-image post=content/go/request_coalescing.md file=/tmp/diagram.png name=singleflight-flow"; \
		exit 1; \
	fi
	go run ./scripts/media --post "$(post)" --file "$(file)" --name "$(name)" --wrangler "$(WRANGLER)"
