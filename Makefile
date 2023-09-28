.PHONY: init
init:
	@git submodule update --init --recursive

lint:
	


.PHONY: update
update:
	@git submodule update --remote --merge


.PHONY: devserver
devserver:
	@hugo server --disableFastRender -e production --bind 0.0.0.0 --ignoreCache
