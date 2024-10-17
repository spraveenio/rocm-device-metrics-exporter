TO_GEN := internal/amdgpu/proto
TO_MOCK := internal/amdgpu/mock
OUT_DIR := bin
export BUILD_CONTAINER ?= registry.test.pensando.io:5000/metrics-exporter-bld:1

TOP_DIR := $(PWD)
GEN_DIR := $(TOP_DIR)/internal/amdgpu/
MOCK_DIR := ${TOP_DIR}/internal/amdgpu/mock_gen
GOINSECURE='github.com, google.golang.org, golang.org'
GOFLAGS ='-buildvcs=false'
BUILD_DATE ?= $(shell date   +%Y-%m-%dT%H:%M:%S%z)
GIT_COMMIT ?= $(shell git rev-list -1 HEAD --abbrev-commit)
VERSION ?=$(RELEASE)

export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}
export ${TOP_DIR}
export ${GOFLAGS}
export ${GOINSECURE}

ASSETS_PATH :=${TOP_DIR}/assets
GPUAGENT_LIBS := ${ASSETS_PATH}/amd_smi_lib/x86_64/lib
THIRDPARTY_LIBS := ${ASSETS_PATH}/thirdparty/x86_64-linux-gnu/lib
PKG_PATH := ${TOP_DIR}/pkg/usr/local/bin
PKG_LIB_PATH := ${TOP_DIR}/pkg/usr/local/metrics/
LUA_PROTO := ${TOP_DIR}/internal/amdgpu/proto/luaplugin.proto
PKG_LUA_PATH := ${TOP_DIR}/pkg/usr/local/etc/metrics/slurm

.PHONY: all
all:
	${MAKE} gen amdexporter metricutil

.PHONY: gen
gen: gopkglist
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} GEN_DIR=$(GEN_DIR) || exit 1; done
	@for c in ${TO_MOCK}; do printf "\n+++++++++++++++++ Generating mock $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} MOCK_DIR=$(MOCK_DIR) GEN_DIR=$(GEN_DIR) || exit 1; done

.PHONY: pkg pkg-clean

pkg-clean:
	rm -rf ${TOP_DIR}/bin/*.deb


pkg: pkg-clean
	${MAKE} gen amdexporter-lite
	#copy precompiled libs
	mkdir -p ${PKG_LIB_PATH}
	cp -rvf ${GPUAGENT_LIBS}/ ${PKG_LIB_PATH}
	cp -rvf ${THIRDPARTY_LIBS}/ ${PKG_LIB_PATH}
	#copy and strip files
	mkdir -p ${PKG_PATH}
	gunzip -c ${ASSETS_PATH}/gpuagent_static.bin.gz > ${PKG_PATH}/gpuagent
	chmod +x ${PKG_PATH}/gpuagent
	cd ${PKG_PATH} && strip ${PKG_PATH}/gpuagent
	cp -vf ${LUA_PROTO} ${PKG_LUA_PATH}/plugin.proto
	cp -vf ${ASSETS_PATH}/gpuctl.gobin ${PKG_PATH}/
	cp -vf $(CURDIR)/bin/amd-metrics-exporter ${PKG_PATH}/
	cd ${TOP_DIR}
	dpkg-deb --build pkg ${TOP_DIR}/bin
	#remove copied files
	rm -rf ${PKG_LIB_PATH}
	rm -rf ${PKG_LUA_PATH}/plugin.proto

.PHONY:clean
clean: pkg-clean
	rm -rf internal/amdgpu/gen
	rm -rf bin
	rm -rf docker/obj
	rm -rf docker/*.tgz
	rm -rf docker/*.tar
	rm -rf docker/*.tar.gz

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
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/golang/mock/mockgen@v1.6.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.1
	go install golang.org/x/tools/cmd/goimports@latest

amdexporter-lite:
	@echo "building lite version of metrics exporter"
	go build -C cmd/exporter -ldflags "-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" -o $(CURDIR)/bin/amd-metrics-exporter

amdexporter:
	@echo "building amd metrics exporter"
	CGO_ENABLED=0 go build  -C cmd/exporter -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" -o $(CURDIR)/bin/amd-metrics-exporter

metricutil:
	@echo "building metrics util"
	CGO_ENABLED=0 go build -C cmd/metricutil -o $(CURDIR)/bin/metricutil

.PHONY: docker-cicd
docker-cicd: gen amdexporter
	echo "Building cicd docker for publish"
	${MAKE} -C docker docker-cicd TOP_DIR=$(CURDIR)
	${MAKE} -C docker docker-save TOP_DIR=$(CURDIR)

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

docker-compile:
	docker run --user $(shell id -u):$(shell id -g) --privileged -e "GIT_COMMIT=${GIT_COMMIT}" -e "GIT_VERSION=${GIT_VERSION}" -e "BUILD_DATE=${BUILD_DATE}" -e "GOPATH=/import" -e GOCACHE=/import/src/github.com/pensando/device-metrics-exporter/.cache --rm -v${PWD}/../../../../:/import/ ${BUILD_CONTAINER} bash -c "cd /import/src/github.com/pensando/device-metrics-exporter && make all"

.PHONY: base-image
base-image:
	${MAKE} -C tools/base-image

copyrights:
	GOFLAGS=-mod=mod go run tools/build/copyright/main.go && ./tools/build/check-local-files.sh

.PHONY: slurm-sim
slurm-sim:
	${MAKE} -C internal/slurm/sim
