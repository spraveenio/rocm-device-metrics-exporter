.PHONY: bld_proto
bld_proto:
	@echo "building proto"
	@protoc --proto_path=. --go-grpc_out=$(GEN_DIR) --go_out=$(GEN_DIR) $(shell ls *.proto)

.PHONY: all
all: bld_proto
