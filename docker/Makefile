EXPORTER_IMAGE ?= $(DOCKER_REGISTRY)/$(EXPORTER_IMAGE_NAME):$(EXPORTER_IMAGE_TAG)
# name used for saving the container images as tar.gz
DOCKER_CONTAINER_IMAGE ?= $(EXPORTER_IMAGE_NAME)-$(EXPORTER_IMAGE_TAG)
HOURLY_TAG_LABEL ?= latest

.PHONY: docker
docker:
	@echo "building amd exporter container"
	@TOP_DIR=$(TOP_DIR) MOCK=$(MOCK) OS=RHEL9 ./build_prep_docker.sh
	@docker build --build-arg BASE_IMAGE=$(RHEL_BASE_MIN_IMAGE) -t $(EXPORTER_IMAGE) . -f Dockerfile.exporter-release
	@TOP_DIR=$(TOP_DIR) ./build_post_docker.sh

.PHONY: docker-cicd
docker-cicd:
	@echo "building cicd docker for local publish"
	@echo "HOURLY_TAG : $(HOURLY_TAG_LABEL)"
	@TOP_DIR=$(TOP_DIR) OS=RHEL9 ./build_prep_docker.sh
	@docker build --build-arg BASE_IMAGE=$(RHEL_BASE_MIN_IMAGE) -t $(EXPORTER_IMAGE) --label HOURLY_TAG=$(HOURLY_TAG_LABEL) . -f Dockerfile.exporter-release
	@TOP_DIR=$(TOP_DIR) ./build_post_docker.sh

.PHONY: docker-save
docker-save: ## save the container
	@echo saving docker image to $(DOCKER_CONTAINER_IMAGE).tar.gz
	@docker save $(EXPORTER_IMAGE) | gzip > $(DOCKER_CONTAINER_IMAGE).tar.gz

.PHONY: azure
azure:
	@echo "building cicd docker for local publish"
	@echo "HOURLY_TAG : $(HOURLY_TAG_LABEL)"
	@TOP_DIR=$(TOP_DIR) OS=AZURE3 ./build_prep_docker.sh
	@docker build --build-arg BASE_IMAGE=$(AZURE_BASE_IMAGE) -t $(EXPORTER_IMAGE) --label HOURLY_TAG=$(HOURLY_TAG_LABEL) . -f Dockerfile.azure.linux3.exporter-release
	@TOP_DIR=$(TOP_DIR) ./build_post_docker.sh
