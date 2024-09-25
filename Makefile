TO_GEN := internal/amdgpu/proto
TO_MOCK := internal/amdgpu/mock
OUT_DIR := bin

TOP_DIR := $(PWD)
GEN_DIR := $(TOP_DIR)/internal/amdgpu/
MOCK_DIR := ${TOP_DIR}/internal/amdgpu/mock_gen

export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}
export ${TOP_DIR}

UT_TEST := internal/amdgpu/gpuagent

ASSETS_PATH :=${TOP_DIR}/assets
PKG_PATH := ${TOP_DIR}/pkg/usr/local/bin

.PHONY: all
all:
	${MAKE} gen amdexporter

.PHONY: gen
gen:
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} GEN_DIR=$(GEN_DIR) || exit 1; done
	@for c in ${TO_MOCK}; do printf "\n+++++++++++++++++ Generating mock $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} MOCK_DIR=$(MOCK_DIR) GEN_DIR=$(GEN_DIR) || exit 1; done

.PHONY: pkg pkg-clean

pkg-clean:
	rm -rf pkg/usr

pkg:pkg-clean
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
clean:
	rm -rf pkg/usr
	rm -rf internal/amdgpu/gen
	rm -rf bin
	rm -rf docker/obj
	rm -rf docker/*.tgz

amdexporter-lite:
	@echo "building lite version of metrics exporter"
	go build -C cmd/exporter -ldflags "-s -w" -o $(CURDIR)/bin/amd-metrics-exporter

amdexporter:
	@echo "building amd metrics exporter"
	go build -C cmd/exporter -o $(CURDIR)/bin/amd-metrics-exporter

.PHONY: docker
docker: amdexporter
	${MAKE} -C docker TOP_DIR=$(CURDIR) MOCK=$(MOCK)

.PHONY: docker-mock
docker-mock:
	${MAKE} docker MOCK=1

.PHONY: docker-publish
docker-publish:
	${MAKE} -C docker docker-publish TOP_DIR=$(CURDIR)

ut: gen
	@for c in ${UT_TEST}; do printf "\n+++++++++++++++++ Testing $${c} +++++++++++++++++\n"; PATH=$$PATH go test -v -mod=vendor github.com/pensando/device-metrics-exporter/$${c} || exit 1; done

loadgpu:
	sudo modprobe amdgpu

mod:
	@echo "setting up go mod packages"
	@go mod tidy
	@go mod vendor
