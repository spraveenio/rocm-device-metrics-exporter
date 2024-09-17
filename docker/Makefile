.PHONY:
docker:
	@echo "building amd exporter container"
	TOP_DIR=$(TOP_DIR) ./build_exporter_release_docker.sh -s

docker-publish:
	@echo "publishing amd exporter container"
	TOP_DIR=$(TOP_DIR) ./build_exporter_release_docker.sh -p

