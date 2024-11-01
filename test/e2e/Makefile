.DEFAULT: all
.PHONY: all lint

TEST_ARGS ?= ""

all:
	go clean -testcache && go test -test.timeout=30m -clean-after-test -v $(TEST_ARGS);

lint:
	@go fmt ./...
	@goimports -w ./
	@go vet ./...
