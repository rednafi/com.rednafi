.PHONY: init
init:
	@git submodule update --init --recursive


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
