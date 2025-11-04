.PHONY: help build run stop clean logs proto

IMAGE_NAME := grpc-multiprocess
CONTAINER_NAME := grpc-container

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

proto: ## Generate protobuf code locally
	@echo "Generating protobuf code..."
	@chmod +x generate.sh
	@./generate.sh

build: ## Build the Docker image
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME) .
	@echo "Build complete!"

run: ## Run the container
	@echo "Starting container..."
	docker run -d --name $(CONTAINER_NAME) $(IMAGE_NAME)
	@echo "Container started. Use 'make logs' to view output."

run-interactive: ## Run the container in interactive mode
	@echo "Starting container in interactive mode..."
	docker run --rm --name $(CONTAINER_NAME) $(IMAGE_NAME)

stop: ## Stop the container
	@echo "Stopping container..."
	docker stop $(CONTAINER_NAME) || true
	docker rm $(CONTAINER_NAME) || true
	@echo "Container stopped and removed."

logs: ## Follow container logs
	docker logs -f $(CONTAINER_NAME)

logs-server: ## Follow server logs only
	docker exec $(CONTAINER_NAME) sh -c "tail -f /var/log/grpc-server/current" 2>/dev/null || \
	docker logs $(CONTAINER_NAME) 2>&1 | grep "gRPC Server"

logs-client: ## Follow client logs only
	docker exec $(CONTAINER_NAME) sh -c "tail -f /var/log/grpc-client/current" 2>/dev/null || \
	docker logs $(CONTAINER_NAME) 2>&1 | grep "gRPC Client"

shell: ## Open a shell in the running container
	docker exec -it $(CONTAINER_NAME) sh

clean: stop ## Stop container and remove image
	@echo "Removing image..."
	docker rmi $(IMAGE_NAME) || true
	@echo "Cleanup complete."

rebuild: clean build ## Clean, rebuild, and run
	@echo "Rebuild complete!"

restart: stop run ## Stop and restart the container
	@echo "Restart complete!"
