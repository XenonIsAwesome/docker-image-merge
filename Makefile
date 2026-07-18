.PHONY: build install clean test

PLUGIN_DIR := $(HOME)/.docker/cli-plugins
BINARY := docker-image-merge
IMAGE := $(BINARY)-builder

build:
	docker build -t $(IMAGE) .
	docker create --name $(BINARY)-tmp $(IMAGE)
	docker cp $(BINARY)-tmp:/$(BINARY) ./$(BINARY)
	docker rm $(BINARY)-tmp
	@echo "Built ./$(BINARY)"

install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/$(BINARY)
	chmod +x $(PLUGIN_DIR)/$(BINARY)
	@echo "Installed to $(PLUGIN_DIR)/$(BINARY)"
	@echo "Run: docker $(BINARY) --help"

clean:
	rm -f $(BINARY)
	docker rmi $(IMAGE) 2>/dev/null || true

test:
	docker build -t $(IMAGE)-test -f Dockerfile.test .
	docker run --rm $(IMAGE)-test
