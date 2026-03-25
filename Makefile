REGISTRY ?= earthworm
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
TAG ?= $(GIT_SHA)

.PHONY: build-agent build-server build-ui build-all push-all helm-package deploy clean

build-agent:
	docker build -t $(REGISTRY)/agent:$(TAG) -t $(REGISTRY)/agent:latest \
		-f deploy/docker/Dockerfile.agent .

build-server:
	docker build -t $(REGISTRY)/server:$(TAG) -t $(REGISTRY)/server:latest \
		-f deploy/docker/Dockerfile.server .

build-ui:
	docker build -t $(REGISTRY)/ui:$(TAG) -t $(REGISTRY)/ui:latest \
		-f deploy/docker/Dockerfile.ui .

build-all: build-agent build-server build-ui

push-all:
	docker push $(REGISTRY)/agent:$(TAG)
	docker push $(REGISTRY)/agent:latest
	docker push $(REGISTRY)/server:$(TAG)
	docker push $(REGISTRY)/server:latest
	docker push $(REGISTRY)/ui:$(TAG)
	docker push $(REGISTRY)/ui:latest

helm-package:
	helm package deploy/helm/earthworm

deploy:
	kubectl apply -f deploy/earthworm.yaml

clean:
	docker rmi $(REGISTRY)/agent:$(TAG) $(REGISTRY)/agent:latest 2>/dev/null || true
	docker rmi $(REGISTRY)/server:$(TAG) $(REGISTRY)/server:latest 2>/dev/null || true
	docker rmi $(REGISTRY)/ui:$(TAG) $(REGISTRY)/ui:latest 2>/dev/null || true
