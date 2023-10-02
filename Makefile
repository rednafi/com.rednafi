.PHONY: init
init:
	@git submodule update --init --recursive
ifndef CI
	@command -v pre-commit > /dev/null || brew install pre-commit
	@pre-commit install
else ifeq ($(CI),false)
	@command -v pre-commit > /dev/null || brew install pre-commit
	@pre-commit install
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
