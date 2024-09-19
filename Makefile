TO_GEN := internal/amdgpu/proto
OUT_DIR := bin

TOP_DIR := $(PWD)

export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}
ASSETS_PATH :=${TOP_DIR}/assets
PKG_PATH := ${TOP_DIR}/pkg/usr/local/bin

.PHONY: all
all:
	${MAKE} gen amdexporter

.PHONY: gen
gen:
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} || exit 1; done

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


