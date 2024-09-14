SHELL := /bin/bash -ex
MAKEFLAGS += --silent

# Define packages as a space-separated list
BREW_PACKAGES := gh hugo pre-commit prettier python@3.12 uv


init:
	git submodule update --init --recursive
	npm install
	npm install -g wrangler
	for pkg in $(BREW_PACKAGES); do \
		brew list $$pkg &>/dev/null || brew install $$pkg; \
	done
	uv venv -p 3.12
	. .venv/bin/activate
	uv pip install black blacken-docs mypy pytest pytest-cov ruff

lint:
	pre-commit run --all-files
	prettier --write .

update:
	git submodule update --remote --merge
	pre-commit autoupdate -j 4
	npm update

devserver:
	hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache

upload-static:
	oxipng -o 6 -r static/images/
	find static -type f | while read filepath; do \
		key=$$(echo "$$filepath" | sed 's|^|blog/|'); \
		wrangler r2 object put $$key --file "$$filepath"; \
	done
