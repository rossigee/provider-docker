# Image building makefile for provider-docker

# Set image-specific variables
IMAGE_NAME := provider-docker

# Include the common image makefile
include ../../../build/makelib/imagelight.mk

# Override img.build target for Docker buildx
img.build:
	@echo "Building Docker image $(IMAGE) for platform $(IMAGE_PLATFORMS)"
	docker buildx build \
		--platform $(IMAGE_PLATFORMS) \
		--tag $(IMAGE) \
		--load \
		.

# Add img.publish target for registry publishing
img.publish: img.build
	@echo "Publishing Docker image $(IMAGE) to registry"
	docker buildx build \
		--platform $(IMAGE_PLATFORMS) \
		--tag $(IMAGE) \
		--push \
		.

.PHONY: img.build img.publish