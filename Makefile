.PHONY: run
run:
ifndef FILE
	@echo "usage: make run FILE=<path-or-directory> [ARGS='-f pattern --stats']"
	@exit 1
endif
	go run ./cmd/vfs $(FILE) $(ARGS)

VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

HOST_OS := $(shell go env GOOS)

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

.PHONY: preflight
preflight:
	@command -v go >/dev/null 2>&1 || { \
		echo "error: go is not installed or not on PATH"; \
		echo "  install Go 1.24+ from https://go.dev/dl/"; \
		exit 1; \
	}
	@GO_VER=$$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//'); \
	MAJOR=$$(echo "$$GO_VER" | cut -d. -f1); \
	MINOR=$$(echo "$$GO_VER" | cut -d. -f2); \
	if [ "$$MAJOR" -lt 1 ] || { [ "$$MAJOR" -eq 1 ] && [ "$$MINOR" -lt 24 ]; }; then \
		echo "error: Go 1.24+ required (found go$$GO_VER)"; \
		exit 1; \
	fi
	@if [ "$$(go env CGO_ENABLED)" != "1" ]; then \
		echo "error: CGO is disabled but vfs requires CGO_ENABLED=1 (tree-sitter C bindings)"; \
		echo "  run: export CGO_ENABLED=1"; \
		exit 1; \
	fi
	@if [ "$(HOST_OS)" = "windows" ]; then \
		command -v gcc >/dev/null 2>&1 || { \
			echo "error: no C compiler found -- vfs requires gcc for tree-sitter"; \
			echo ""; \
			echo "  install MinGW-w64 via MSYS2:"; \
			echo "    pacman -S mingw-w64-x86_64-gcc"; \
			echo "  then add C:\\msys64\\mingw64\\bin to PATH"; \
			echo ""; \
			echo "  or install TDM-GCC: https://jmeubank.github.io/tdm-gcc/"; \
			exit 1; \
		}; \
	else \
		if ! cc -v >/dev/null 2>&1; then \
			echo "error: no C compiler found -- vfs requires one for tree-sitter"; \
			if [ "$$(uname)" = "Darwin" ]; then \
				echo ""; \
				echo "  on macOS, install Xcode Command Line Tools:"; \
				echo "    xcode-select --install"; \
				echo ""; \
				echo "  if already installed but license not accepted:"; \
				echo "    sudo xcodebuild -license accept"; \
			else \
				echo "  install gcc or clang (e.g. apt install build-essential)"; \
			fi; \
			exit 1; \
		fi; \
	fi

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

.PHONY: build
ifeq ($(HOST_OS),windows)
build: preflight
	go build $(LDFLAGS) -o bin/vfs.exe ./cmd/vfs
else
build: preflight
	go build $(LDFLAGS) -o bin/vfs ./cmd/vfs
endif

.PHONY: build-windows
build-windows: preflight-cross-windows
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o bin/vfs.exe ./cmd/vfs

.PHONY: preflight-cross-windows
preflight-cross-windows:
	@command -v go >/dev/null 2>&1 || { \
		echo "error: go is not installed or not on PATH"; \
		echo "  install Go 1.24+ from https://go.dev/dl/"; \
		exit 1; \
	}
	@command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1 || { \
		echo "error: mingw-w64 cross-compiler not found"; \
		echo "  macOS:   brew install mingw-w64"; \
		echo "  Linux:   apt install gcc-mingw-w64-x86-64"; \
		exit 1; \
	}

# ---------------------------------------------------------------------------
# Install
# ---------------------------------------------------------------------------

INSTALL_DIR ?= $(shell go env GOPATH)/bin

.PHONY: install
ifeq ($(HOST_OS),windows)
install: build
	@mkdir -p $(INSTALL_DIR)
	@taskkill /F /IM vfs.exe >NUL 2>&1 || true
	@sleep 1
	@rm -f "$(INSTALL_DIR)/vfs.exe"
	@cp bin/vfs.exe "$(INSTALL_DIR)/vfs.exe"
	@echo "vfs installed to $(INSTALL_DIR)/vfs.exe"
else
install: build
	@mkdir -p $(INSTALL_DIR)
	@if [ -f $(VFS_PID) ] && kill -0 $$(cat $(VFS_PID)) 2>/dev/null; then \
		kill $$(cat $(VFS_PID)) 2>/dev/null; \
		rm -f $(VFS_PID); \
		echo "stopped running vfs server"; \
	fi
	@pkill -x vfs 2>/dev/null && echo "killed running vfs processes" || true
	@sleep 0.5
	@rm -f $(INSTALL_DIR)/vfs
	@cp bin/vfs $(INSTALL_DIR)/vfs
	@chmod +x $(INSTALL_DIR)/vfs
	@xattr -c $(INSTALL_DIR)/vfs 2>/dev/null || true
	@echo "vfs installed to $(INSTALL_DIR)/vfs"
endif

# ---------------------------------------------------------------------------
# Release
# ---------------------------------------------------------------------------

.PHONY: release-tag
release-tag:
	@if git rev-parse "v$(VERSION)" >/dev/null 2>&1; then \
		echo "tag v$(VERSION) already exists"; exit 1; \
	fi
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"
	@echo "pushed tag v$(VERSION) — GitHub Actions will create the release"

# ---------------------------------------------------------------------------
# Lint / Test
# ---------------------------------------------------------------------------

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

# ---------------------------------------------------------------------------
# Bench
# ---------------------------------------------------------------------------

.PHONY: bench
bench: build
	@echo ""
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe bench --self
else
	@./bin/vfs bench --self
endif
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
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe bench -f "$(PATTERN)" "$(DIR)"
else
	@./bin/vfs bench -f "$(PATTERN)" "$(DIR)"
endif

# ---------------------------------------------------------------------------
# Dashboard
# ---------------------------------------------------------------------------

.PHONY: dashboard
dashboard: build
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe dashboard
else
	@./bin/vfs dashboard
endif

# ---------------------------------------------------------------------------
# Server management
# ---------------------------------------------------------------------------

ifeq ($(HOST_OS),windows)
VFS_LOG ?= $(shell echo %TEMP%)/vfs-serve.log
VFS_PID  = $(shell echo %TEMP%)/vfs-serve.pid
else
VFS_LOG ?= /tmp/vfs-serve.log
VFS_PID  = /tmp/vfs-serve.pid
endif

.PHONY: serve
serve: build
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe serve
else
	@./bin/vfs serve
endif

.PHONY: up
up: build
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe up
else
	@if [ -f $(VFS_PID) ] && kill -0 $$(cat $(VFS_PID)) 2>/dev/null; then \
		echo "vfs is already running (pid $$(cat $(VFS_PID)))"; \
		echo "  dashboard: http://localhost:3000"; \
		echo "  MCP:       http://localhost:8080/mcp"; \
	else \
		nohup ./bin/vfs serve > $(VFS_LOG) 2>&1 & echo $$! > $(VFS_PID); \
		echo "vfs started (pid $$(cat $(VFS_PID)))"; \
		echo "  dashboard: http://localhost:3000"; \
		echo "  MCP:       http://localhost:8080/mcp"; \
		echo "  log:       $(VFS_LOG)"; \
		echo "  stop:      make down"; \
	fi
endif

.PHONY: down
down:
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe down
else
	@if [ -f $(VFS_PID) ] && kill -0 $$(cat $(VFS_PID)) 2>/dev/null; then \
		kill $$(cat $(VFS_PID)); \
		rm -f $(VFS_PID); \
		echo "vfs stopped"; \
	else \
		rm -f $(VFS_PID); \
		echo "vfs is not running"; \
	fi
endif

.PHONY: status
status:
ifeq ($(HOST_OS),windows)
	@./bin/vfs.exe status
else
	@if [ -f $(VFS_PID) ] && kill -0 $$(cat $(VFS_PID)) 2>/dev/null; then \
		echo "vfs is running (pid $$(cat $(VFS_PID)))"; \
		echo "  dashboard: http://localhost:3000"; \
		echo "  MCP:       http://localhost:8080/mcp"; \
	else \
		echo "vfs is not running"; \
	fi
endif

# ---------------------------------------------------------------------------
# Docker
# ---------------------------------------------------------------------------

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

# ---------------------------------------------------------------------------
# Clean / Help
# ---------------------------------------------------------------------------

.PHONY: clean
ifeq ($(HOST_OS),windows)
clean:
	@rm -f bin/vfs.exe
else
clean: down
	rm -f bin/vfs bin/vfs.exe
endif

.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  run FILE=<path> [ARGS='...']           - Run vfs on a file or directory"
	@echo "  preflight                              - Check Go, CGO, and C compiler"
	@echo "  build                                  - Build binary (vfs or vfs.exe)"
	@echo "  build-windows                          - Cross-compile Windows binary to bin/vfs.exe"
	@echo "  install [INSTALL_DIR=/usr/local/bin]   - Build and copy binary to INSTALL_DIR"
	@echo "  bench                                  - Quick self-test benchmark"
	@echo "  bench-on DIR=<path> PATTERN=<pattern>  - Benchmark on any project"
	@echo "  test                                   - Run tests"
	@echo "  test-coverage                          - Run tests with coverage"
	@echo "  test-race                              - Run tests with race detection"
	@echo "  lint                                   - Run linter"
	@echo "  dashboard                              - Build and open dashboard on :3000"
	@echo "  serve                                  - Run MCP server + dashboard (foreground)"
	@echo "  up                                     - Start MCP server + dashboard (detached)"
	@echo "  down                                   - Stop detached server"
	@echo "  status                                 - Check if server is running"
	@echo "  docker-build                           - Build Docker image (vfs-mcp)"
	@echo "  docker-run                             - Run MCP server + dashboard in Docker"
	@echo "  docker-cli ARGS='<path> [flags]'       - Run vfs as CLI binary in Docker"
	@echo "  release-tag                            - Tag v$$(cat VERSION) and push (triggers release)"
	@echo "  clean                                  - Remove build artifacts"
	@echo "  help                                   - Show this help message"
	@echo ""
	@echo "Supported languages: Go, JavaScript, TypeScript, JSX, TSX, Python,"
	@echo "                     Rust, Java, HCL/Terraform, Dockerfile, Protobuf, SQL, YAML"
	@echo ""
	@echo "Quick start:"
	@echo "  make bench                                       # self-test"
	@echo "  make bench-on DIR=~/projects/myapp PATTERN=Login # your project"
	@echo ""
