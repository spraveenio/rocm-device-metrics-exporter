DOCKER_REGISTRY ?= registry.test.pensando.io:5000/test-runner
IMAGE_NAME ?= test-runner
IMAGE_TAG_BASE ?= $(DOCKER_REGISTRY)/$(IMAGE_NAME)
# This is the default tag of all images made by this Makefile.
IMAGE_TAG ?= dev
# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(IMAGE_TAG)

# name used for saving the container images as tar.gz
DOCKER_CONTAINER_IMG ?= $(IMAGE_NAME)-$(IMAGE_TAG)
HOURLY_TAG_LABEL ?= latest

.PHONY: docker
docker:
	@echo "building amd testrunner container"
	@cp $(TOP_DIR)/bin/amd-test-runner .
	@docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubi9/ubi:9.4 -t ${IMG} . -f $(TOP_DIR)/docker/testrunner/Dockerfile
	@rm -rf $(TOP_DIR)/docker/testrunner/amd-test-runner

.PHONY: docker-cicd
docker-cicd:
	@echo "building amd testrunner container"
	@echo "HOURLY_TAG : $(HOURLY_TAG_LABEL)"
	@cp $(TOP_DIR)/bin/amd-test-runner .
	@docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubi9/ubi:9.4 -t ${IMG} --label HOURLY_TAG=$(HOURLY_TAG_LABEL) . -f $(TOP_DIR)/docker/testrunner/Dockerfile
	@rm -rf $(TOP_DIR)/docker/testrunner/amd-test-runner

.PHONY: docker-save
docker-save: ## save the container
	@echo "saving docker image to $(DOCKER_CONTAINER_IMG).tar.gz"
	@docker save $(IMG) | gzip > $(DOCKER_CONTAINER_IMG).tar.gz