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
	@go build $(TOP_DIR)/cmd/testrunner/
	@cp $(TOP_DIR)/cmd/testrunner/testrunner .
	@docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubuntu:22.04 -t ${IMG} . -f $(TOP_DIR)/docker/testrunner/Dockerfile