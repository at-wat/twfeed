.PHONY: build
build:
	docker build -t twfeed:latest .

.PHONY: run
run:
	docker run -it --rm \
		-v "$(PWD)/cookies.json:/cookies.json:ro" \
		--env-file .env \
		twfeed:latest
