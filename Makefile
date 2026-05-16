VERSION ?= dev
IMAGE   ?= sshelf:$(VERSION)

.PHONY: build test lint shell try clean release

build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

test:
	docker compose run --rm test

lint:
	docker compose run --rm lint

shell:
	docker compose run --rm dev

try:
	docker compose run --rm try profile init

release:
	docker buildx build \
	  --platform linux/amd64,linux/arm64 \
	  --build-arg VERSION=$(VERSION) \
	  -t $(IMAGE) \
	  --output=type=local,dest=./dist \
	  .

clean:
	docker compose down -v
	rm -rf dist/