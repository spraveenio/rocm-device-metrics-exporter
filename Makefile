TO_GEN := internal/amdgpu/proto
TO_MOCK := internal/amdgpu/mock
OUT_DIR := bin

TOP_DIR := $(PWD)
GEN_DIR := $(TOP_DIR)/internal/amdgpu/
MOCK_DIR := ${TOP_DIR}/internal/amdgpu/mock_gen
GOINSECURE='github.com, google.golang.org, golang.org'
GOFLAGS ='-buildvcs=false'

export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}
export ${TOP_DIR}
export ${GOFLAGS}
export ${GOINSECURE}

ASSETS_PATH :=${TOP_DIR}/assets
PKG_PATH := ${TOP_DIR}/pkg/usr/local/bin

.PHONY: all
all:
	${MAKE} gen amdexporter

.PHONY: gen
gen: gopkglist
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} GEN_DIR=$(GEN_DIR) || exit 1; done
	@for c in ${TO_MOCK}; do printf "\n+++++++++++++++++ Generating mock $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} MOCK_DIR=$(MOCK_DIR) GEN_DIR=$(GEN_DIR) || exit 1; done

.PHONY: pkg pkg-clean

pkg-clean:
	rm -rf ${TOP_DIR}/bin/*.deb


pkg: pkg-clean
	${MAKE} gen amdexporter-lite
	#copy and strip files
	mkdir -p ${PKG_PATH}
	gunzip -c ${ASSETS_PATH}/gpuagent_static.bin.gz > ${PKG_PATH}/gpuagent
	chmod +x ${PKG_PATH}/gpuagent
	cd ${PKG_PATH} && strip ${PKG_PATH}/gpuagent
	cp -vf ${ASSETS_PATH}/gpuctl.gobin ${PKG_PATH}/
	cp -vf $(CURDIR)/bin/amd-metrics-exporter ${PKG_PATH}/
	cd ${TOP_DIR}
	dpkg-deb --build pkg ${TOP_DIR}/bin

.PHONY:clean
clean: pkg-clean
	rm -rf internal/amdgpu/gen
	rm -rf bin
	rm -rf docker/obj
	rm -rf docker/*.tgz

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.1)

# go-get-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
}
endef

GOFILES_NO_VENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
.PHONY: lint
lint: golangci-lint ## Run golangci-lint against code.
	@if [ `gofmt -l $(GOFILES_NO_VENDOR) | wc -l` -ne 0 ]; then \
		echo There are some malformed files, please make sure to run \'make fmt\'; \
		gofmt -l $(GOFILES_NO_VENDOR); \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run -v --timeout 5m0s

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: gopkglist
gopkglist:
	go install github.com/golang/protobuf/protoc-gen-go@v1.5.4
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/golang/mock/mockgen@v1.6.0
	go install golang.org/x/tools/cmd/goimports@v0.25.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.1

amdexporter-lite:
	@echo "building lite version of metrics exporter"
	go build -C cmd/exporter -ldflags "-s -w" -o $(CURDIR)/bin/amd-metrics-exporter

amdexporter:
	@echo "building amd metrics exporter"
	go build -C cmd/exporter -o $(CURDIR)/bin/amd-metrics-exporter

.PHONY: docker
docker: gen amdexporter
	${MAKE} -C docker TOP_DIR=$(CURDIR) MOCK=$(MOCK)

.PHONY: docker-mock
docker-mock:
	${MAKE} docker MOCK=1

.PHONY:checks
checks: gen vet

.PHONY: docker-publish
docker-publish:
	${MAKE} -C docker docker-publish TOP_DIR=$(CURDIR)


.PHONY: unit-test
unit-test:
	PATH=$$PATH LOGDIR=$(TOP_DIR)/ go test -v -cover -mod=vendor ./...

loadgpu:
	sudo modprobe amdgpu

mod:
	@echo "setting up go mod packages"
	@go mod tidy
	@go mod vendor
