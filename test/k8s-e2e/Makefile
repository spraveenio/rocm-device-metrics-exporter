.DEFAULT: all
.PHONY: all lint

TEST_ARGS :=
# define custom kubeconfig file path below. default will be $(HOME)/.kube/config
KUBECONFIG :=

ifdef KUBECONFIG
	TEST_ARGS += -kubeconfig=$(KUBECONFIG)
endif

all:
	go test -failfast -helmchart $(TOP_DIR)/helm-charts/ -test.timeout=30m -v $(TEST_ARGS); 

lint:
	@go fmt ./...
	@goimports -w ./
	@go vet ./...
