#can not exist space
cp-deps:
	@mkdir -p bin config
	@make --no-print-directory -C vendor/github.com/spinlock/jemalloc-go/
build: cp-deps
	go build -i  -o bin/rdslive-router ./cmd/proxy
	@./bin/rdslive-router --default-config > config/router.toml
image:build
	docker build --force-rm -t $(REGISTY)/$(PRODUCT):$(VERSION) .
clean:
	@rm -rf bin
distclean: clean
	@make --no-print-directory --quiet -C vendor/github.com/spinlock/jemalloc-go/ distclean
.PHONY:build