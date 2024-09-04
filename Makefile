define PRINT_STEP
	@echo "$$(tput setaf 6)$$(tput bold)\n:> $1\n$$(tput sgr0)"
endef

# Define packages as a space-separated list
BREW_PACKAGES := gh hugo pre-commit prettier python@3.12 uv

init:
	@git submodule update --init --recursive
	@npm install

ifeq ($(CI),)
	@$(call PRINT_STEP,installing brew dependencies)
	@for pkg in $(BREW_PACKAGES); do \
		brew list $$pkg &>/dev/null || (echo "Installing $$pkg..." && brew install $$pkg); \
	done

	@$(call PRINT_STEP,creating python venv)
	@uv venv -p 3.12

	@$(call PRINT_STEP,installing python dependencies)
	@. .venv/bin/activate
	@uv pip install black blacken-docs mypy pytest pytest-cov ruff

	@$(call PRINT_STEP,initialization complete)
endif


lint:
	@pre-commit run --all-files
	@prettier --write .


update:
	@git submodule update --remote --merge
	@pre-commit autoupdate -j 4
	@npm update


devserver:
	@hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache
