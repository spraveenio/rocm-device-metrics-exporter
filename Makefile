TO_GEN := proto
OUT_DIR := bin


export ${GOROOT}
export ${GOPATH}
export ${OUT_DIR}

.PHONY: gen
gen:
	@for c in ${TO_GEN}; do printf "\n+++++++++++++++++ Generating $${c} +++++++++++++++++\n"; PATH=$$PATH make -C $${c} || exit 1; done

.PHONY:clean
clean:
	rm -rf gen
	rm -rf internal/bin

amdexporter:
	${MAKE} -C internal

.PHONY: docker
docker:
	${MAKE} -C docker TOP_DIR=$(CURDIR)

.PHONY: all
all:
	${MAKE} gen amdexporter

