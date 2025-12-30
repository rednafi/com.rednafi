# Make all the rules since we don't use make as a build tool
.PHONY: $(MAKECMDGOALS)

SHELL := /bin/bash -ex
MAKEFLAGS += --silent

# Define packages as a space-separated list
BREW_PACKAGES := gh hugo prettier uv

init:
	git submodule update --init --recursive
	npm install
	npm install -g wrangler
	for pkg in $(BREW_PACKAGES); do \
		brew list $$pkg &>/dev/null || brew install $$pkg; \
	done
	uv venv -p 3.14
	uv tool install pre-commit
	uv pip install black blacken-docs mypy pytest pytest-cov ruff

format:
	./scripts/lint-codeblocks.sh
	git status --porcelain | awk '{print $$2}' | xargs -r uvx pre-commit run --files
	git status --porcelain | awk '{print $$2}' | grep '.md' | xargs -n 1 prettier --write

update:
	git submodule update --remote --merge
	uvx pre-commit autoupdate -j 4
	npm update

dev:
	hugo --environment production --minify --gc --cleanDestinationDir
	npx -y pagefind --site public
	hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache

build:
	hugo --environment production --minify --gc --cleanDestinationDir
	npx -y pagefind --site public

upload-image:
	@if [ -z "$(local_path)" ] || [ -z "$(remote_path)" ]; then \
		echo "Usage: make upload-image local_path=<file> remote_path=<bucket_path>"; \
		echo ""; \
		echo "Examples:"; \
		echo "  make upload-image local_path=static/images/transparent.png remote_path=blog/static/images/transparent.png"; \
		echo "  make upload-image local_path=content/images/cover.png remote_path=blog/static/images/home/cover.png"; \
		exit 1; \
	fi
	oxipng -o 6 $(local_path)
	wrangler r2 object put $(remote_path) --file $(local_path) --remote
