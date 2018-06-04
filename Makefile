#can not exist space
cp-deps:
	@mkdir -p bin config
build: cp-deps
	go build -i -o bin/rds-router ./cmd/proxy
	@./bin/rds-router --default-config > config/router.toml
image:build
	docker build --force-rm -t $(REGISTY)/$(PRODUCT):$(VERSION) .
distclean:
	@rm -rf bin
install:
	@cp bin/rds-router /usr/bin
	@cp config/router.toml /etc
.PHONY:build