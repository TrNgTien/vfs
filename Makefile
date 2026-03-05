.PHONY: run
run:
ifndef FILE
	@echo "usage: make run FILE=<path-or-directory> [ARGS='-f pattern --stats']"
	@exit 1
endif
	go run ./cmd/vfs $(FILE) $(ARGS)

.PHONY: build
build:
	go build -o bin/vfs ./cmd/vfs

.PHONY: install
install:
	go install ./cmd/vfs

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: lint-fix
lint-fix:
	@golangci-lint run --fix ./...

.PHONY: lint-fmt
lint-fmt:
	@golangci-lint fmt 

.PHONY: test
test:
	@go test ./...

.PHONY: test-coverage
test-coverage:
	@go test -cover ./...
	@go tool cover -html=coverage.out

.PHONY: test-race
test-race:
	@go test -race ./...

.PHONY: bench
bench: build
	@echo ""
	@./bin/vfs bench --self
	@echo ""

.PHONY: bench-on
bench-on: build
ifndef DIR
	@echo "usage: make bench-on DIR=/path/to/project PATTERN=funcName"
	@exit 1
endif
ifndef PATTERN
	@echo "usage: make bench-on DIR=/path/to/project PATTERN=funcName"
	@exit 1
endif
	@./bin/vfs bench -f "$(PATTERN)" "$(DIR)"

.PHONY: dashboard
dashboard: build
	@./bin/vfs dashboard

.PHONY: serve
serve: build
	@./bin/vfs serve

DOCKER_IMAGE ?= vfs-mcp

.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run: docker-build
	docker run --rm -v "$$(pwd):/workspace" -p 8080:8080 -p 3000:3000 $(DOCKER_IMAGE)

.PHONY: docker-cli
docker-cli: docker-build
ifndef ARGS
	@echo "usage: make docker-cli ARGS='<path> [flags]'"
	@echo "  e.g. make docker-cli ARGS='/workspace -f HandleLogin'"
	@echo "       make docker-cli ARGS='stats'"
	@exit 1
endif
	docker run --rm -v "$$(pwd):/workspace" $(DOCKER_IMAGE) $(ARGS)

.PHONY: clean
clean:
	rm -f bin/vfs

.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  run FILE=<path> [ARGS='...']           - Run vfs on a file or directory"
	@echo "  build                                  - Build binary to bin/vfs"
	@echo "  install                                - Install vfs to GOBIN (go install)"
	@echo "  bench                                  - Quick self-test benchmark"
	@echo "  bench-on DIR=<path> PATTERN=<pattern>  - Benchmark on any project"
	@echo "  test                                   - Run tests"
	@echo "  test-coverage                          - Run tests with coverage"
	@echo "  test-race                              - Run tests with race detection"
	@echo "  lint                                   - Run linter"
	@echo "  dashboard                              - Build and open dashboard on :3000"
	@echo "  serve                                  - Run MCP server + dashboard locally"
	@echo "  docker-build                           - Build Docker image (vfs-mcp)"
	@echo "  docker-run                             - Run MCP server + dashboard in Docker"
	@echo "  docker-cli ARGS='<path> [flags]'       - Run vfs as CLI binary in Docker"
	@echo "  clean                                  - Remove build artifacts"
	@echo "  help                                   - Show this help message"
	@echo ""
	@echo "Supported languages: Go, JavaScript, TypeScript, JSX, TSX, Python,"
	@echo "                     HCL/Terraform, Dockerfile, Protobuf, SQL, YAML"
	@echo ""
	@echo "Quick start:"
	@echo "  make bench                                       # self-test"
	@echo "  make bench-on DIR=~/projects/myapp PATTERN=Login # your project"
	@echo ""
