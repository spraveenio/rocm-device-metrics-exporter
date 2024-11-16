.DEFAULT: all
.PHONY: all lint

TEST_ARGS = ""

all:
	@if [ -z ${KUBECONFIG} ]; then \
		TEST_ARGS += "-kubeconfig $(KUBECONFIG) " \
		go test -failfast -helmchart $(TOP_DIR)/helm-charts/ -test.timeout=30m -v $(TEST_ARGS); \
	else \
		go test -failfast -helmchart $(TOP_DIR)/helm-charts/ -test.timeout=30m -v ; \
	fi

lint:
	@go fmt ./...
	@goimports -w ./
	@go vet ./...
