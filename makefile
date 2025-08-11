BINARY_NAME=psygateway
PRODUCT_SERVICE=productservice
USER_SERVICE=userservice

.PHONY: build clean dep run-gateway run-services

build:
	@mkdir -p ./build
	go build -o ./build/$(BINARY_NAME) main.go
	@echo "Build complete: ./build/$(BINARY_NAME)"
	go build -o ./build/$(PRODUCT_SERVICE) ./services/products/main.go
	go build -o ./build/$(USER_SERVICE) ./services/users/main.go
	@echo "Build complete for services: ./build/$(PRODUCT_SERVICE) and ./build/$(USER_SERVICE)"

clean:
	go clean
	@echo "Cleaned Go build cache"
	rm -rf ./build
	@echo "Cleaned build directory"

dep:
	go mod download
	@echo "Dependencies downloaded"

# Optional: Add convenience targets
run-psygateway: build
	./build/$(BINARY_NAME)

run-user-service: build
	./build/$(USER_SERVICE)
	@echo "User service running: ./build/$(USER_SERVICE)"

run-product-service: build
	./build/$(PRODUCT_SERVICE)
	@echo "Product service running: ./build/$(PRODUCT_SERVICE)"