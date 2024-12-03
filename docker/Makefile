DOCKER_REGISTRY ?= registry.test.pensando.io:5000/device-metrics-exporter
IMAGE_NAME ?= exporter
IMAGE_TAG_BASE ?= $(DOCKER_REGISTRY)/$(IMAGE_NAME)
# This is the default tag of all images made by this Makefile.
IMAGE_TAG ?= dev
# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(IMAGE_TAG)

# name used for saving the container images as tar.gz
DOCKER_CONTAINER_IMG = $(IMAGE_NAME)-$(IMAGE_TAG)
HOURLY_TAG_LABEL ?= latest

.PHONY: docker
docker:
	@echo "building amd exporter container"
	TOP_DIR=$(TOP_DIR) MOCK=$(MOCK) ./build_exporter_release_docker.sh -s

.PHONY:docker-publish
docker-publish:
	@echo "publishing amd exporter container"
	TOP_DIR=$(TOP_DIR) ./build_exporter_release_docker.sh -p


.PHONY: docker-cicd
docker-cicd:
	@echo "building cicd docker for local publish"
	@echo "HOURLY_TAG : $(HOURLY_TAG_LABEL)"
	TOP_DIR=$(TOP_DIR) ./build_prep_docker.sh
	docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubi9/ubi-minimal:9.4 -t $(IMG) --label HOURLY_TAG=$(HOURLY_TAG_LABEL) . -f Dockerfile.exporter-release
	TOP_DIR=$(TOP_DIR) ./build_post_docker.sh

.PHONY: docker-save
docker-save: ## save the container
	docker save $(IMG) | gzip > $(DOCKER_CONTAINER_IMG).tar.gz

.PHONY: azure
azure:
	@echo "building cicd docker for local publish"
	@echo "HOURLY_TAG : $(HOURLY_TAG_LABEL)"
	TOP_DIR=$(TOP_DIR) ./build_prep_docker.sh
	docker build -t $(IMG) --label HOURLY_TAG=$(HOURLY_TAG_LABEL) . -f Dockerfile.azure.linux3.exporter-release
	TOP_DIR=$(TOP_DIR) ./build_post_docker.sh
