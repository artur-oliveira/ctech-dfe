BINARY      := ctech-dfe-api
BUILD_DIR   := dist
GOOS        ?= linux
GOARCH      ?= arm64
CGO_ENABLED := 0

.PHONY: build build-worker lint test test-integration vet clean deploy

build:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/server

lint:
	golangci-lint run ./...

test:
	go test ./... -race -coverprofile=coverage.out

# Requires DynamoDB Local: docker compose -f docker-compose.test.yml up -d
test-integration:
	DYNAMODB_ENDPOINT=http://localhost:8123 \
		go test -tags integration -race -count=1 ./tests/integration/ -v

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)

# Deploy to EC2: copy binary + env + systemd unit, restart service
# Usage: make deploy HOST=10.0.1.5
deploy: build
	rsync -az $(BUILD_DIR)/$(BINARY) $(HOST):/opt/ctech-dfe-api/$(BINARY)
	rsync -az ctech-dfe-api.service $(HOST):/etc/systemd/system/
	ssh $(HOST) "sudo systemctl daemon-reload && sudo systemctl restart ctech-dfe-api"
