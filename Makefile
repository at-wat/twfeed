IMAGE     := ghcr.io/at-wat/twfeed
IMAGE_TAG := latest

.PHONY: build
build:
	docker build -t $(IMAGE):$(IMAGE_TAG) .

.PHONY: run
run:
	docker run -it --rm \
		-v "$(PWD)/cookies.json:/cookies.json:ro" \
		--env-file .env \
		$(IMAGE):$(IMAGE_TAG)

.PHONY: show-image-full-name
show-image-full-name:
	@echo "$(IMAGE):$(IMAGE_TAG)"
