TO_GEN := internal/amdgpu/proto
OUT_DIR := bin

export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}

.PHONY: all
all:
	${MAKE} gen amdexporter

.PHONY: gen
gen:
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} || exit 1; done

.PHONY:clean
clean:
	rm -rf internal/amdgpu/gen
	rm -rf bin

amdexporter:
	@echo "buildign amd metrics exporter"
	go build -C cmd/exporter -o $(CURDIR)/bin/amd-metrics-exporter

.PHONY: docker
docker:
	${MAKE} -C docker TOP_DIR=$(CURDIR)

.PHONY: docker-publish
docker-publish:
	${MAKE} -C docker docker-publish TOP_DIR=$(CURDIR)


