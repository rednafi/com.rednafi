define PRINT_STEP
	@echo "$$(tput setaf 6)$$(tput bold):> $1 $$(tput sgr0)"
endef

.PHONY: init
init:
	@git submodule update --init --recursive

ifndef CI
	@$(call PRINT_STEP,installing brew dependencies)
	@brew bundle --force

	@$(call PRINT_STEP,installing pre-commit hooks)
	@pre-commit install

	@$(call PRINT_STEP,creating python venv)
	@python3.12 -m venv .venv

	@$(call PRINT_STEP,updating python dependencies)
	@.venv/bin/pip install pip-tools
	@.venv/bin/pip-compile \
		--config pyproject.toml --extra dev --output-file requirements-dev.txt

	@$(call PRINT_STEP,installing python dependencies)
	@.venv/bin/pip install -r requirements-dev.txt

	@$(call PRINT_STEP,initialization complete)
endif

.PHONY: lint
lint:
	@pre-commit run --all-files


.PHONY: update
update:
	@git submodule update --remote --merge
	@pre-commit autoupdate -j 4


.PHONY: devserver
devserver:
	@hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache
