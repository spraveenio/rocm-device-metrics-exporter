.PHONY: mock_gen
TO_MOCK := scheduler
mock_gen:
	@echo "building mock scheduler"
	@mkdir -p ${MOCK_DIR} 2>/dev/null
	@for c in ${TO_MOCK}; do printf "\n+++++++++++++++++ Generating mock $${c} +++++++++++++++++\n"; mockgen --destination=${MOCK_DIR}/mock_$${c}.go -package=mock_gen --source=$${c}.go; done;
	@goimports -local "github.com/ROCm/device-metrics-exporter" -l ${MOCK_DIR}

.PHONY: all
all: mock_gen
