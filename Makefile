# Movie Data Capture Go Makefile

# 项目配置
PROJECT_NAME := mdc
MAIN_FILE := main.go
VERSION ?= dev
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建配置
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)
BUILD_FLAGS := -ldflags="$(LDFLAGS)"

# 支持的平台
PLATFORMS := \
	windows/amd64 \
	windows/386 \
	windows/arm64 \
	linux/amd64 \
	linux/386 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64

# 默认目标
.PHONY: all
all: clean test build

# 清理构建文件
.PHONY: clean
clean:
	@echo "🧹 清理构建文件..."
	@rm -rf dist/

# 运行测试
.PHONY: test
test:
	@echo "🧪 运行测试..."
	@go test -v ./...

# 下载依赖
.PHONY: deps
deps:
	@echo "📦 下载依赖..."
	@go mod download
	@go mod tidy

# 构建所有平台
.PHONY: build
build: deps
	@echo "🚀 开始构建 $(PROJECT_NAME) $(VERSION)..."
	@mkdir -p dist/
	@$(MAKE) $(addprefix build-, $(PLATFORMS))

# 构建特定平台的模板
.PHONY: build-%
build-%:
	$(eval GOOS := $(word 1,$(subst /, ,$*)))
	$(eval GOARCH := $(word 2,$(subst /, ,$*)))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	$(eval OUTPUT := $(PROJECT_NAME)-$(GOOS)-$(GOARCH)$(EXT))
	$(eval BUILD_DIR := dist/$(PROJECT_NAME)-$(GOOS)-$(GOARCH))
	
	@echo "🔨 构建 $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(OUTPUT) $(MAIN_FILE)
	@cp config.yaml $(BUILD_DIR)/
	@cp README.md $(BUILD_DIR)/
	@cp -r Img $(BUILD_DIR)/ 2>/dev/null || true
	
	# 创建压缩包
	@cd dist/ && \
	if [ "$(GOOS)" = "windows" ]; then \
		if command -v zip >/dev/null 2>&1; then \
			zip -r $(PROJECT_NAME)-$(GOOS)-$(GOARCH).zip $(PROJECT_NAME)-$(GOOS)-$(GOARCH)/ >/dev/null && \
			echo "✅ $(GOOS)/$(GOARCH) 构建完成 (zip)"; \
		else \
			echo "✅ $(GOOS)/$(GOARCH) 构建完成 (未压缩)"; \
		fi \
	else \
		tar -czf $(PROJECT_NAME)-$(GOOS)-$(GOARCH).tar.gz $(PROJECT_NAME)-$(GOOS)-$(GOARCH)/ && \
		echo "✅ $(GOOS)/$(GOARCH) 构建完成 (tar.gz)"; \
	fi

# 构建当前平台
.PHONY: build-current
build-current: deps
	@echo "🔨 构建当前平台..."
	@go build $(BUILD_FLAGS) -o $(PROJECT_NAME) $(MAIN_FILE)
	@echo "✅ 构建完成: $(PROJECT_NAME)"

# 运行程序 (当前平台)
.PHONY: run
run: build-current
	@echo "🚀 运行程序..."
	@./$(PROJECT_NAME)

# 运行程序 (开发模式)
.PHONY: dev
dev:
	@echo "🛠️  开发模式运行..."
	@go run $(MAIN_FILE)

# 代码检查
.PHONY: lint
lint:
	@echo "🔍 运行代码检查..."
	@go vet ./...
	@go fmt ./...

# 安装到系统
.PHONY: install
install: build-current
	@echo "📦 安装到系统..."
	@sudo cp $(PROJECT_NAME) /usr/local/bin/
	@echo "✅ 安装完成: /usr/local/bin/$(PROJECT_NAME)"

# 卸载
.PHONY: uninstall
uninstall:
	@echo "🗑️  卸载程序..."
	@sudo rm -f /usr/local/bin/$(PROJECT_NAME)
	@echo "✅ 卸载完成"

# 显示构建信息
.PHONY: info
info:
	@echo "📋 构建信息:"
	@echo "   项目: $(PROJECT_NAME)"
	@echo "   版本: $(VERSION)"
	@echo "   构建时间: $(BUILD_TIME)"
	@echo "   Git提交: $(GIT_COMMIT)"
	@echo "   Go版本: $(shell go version)"
	@echo "   支持平台: $(PLATFORMS)"

# 显示帮助
.PHONY: help
help:
	@echo "📚 Movie Data Capture Go 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make all           - 清理、测试并构建所有平台"
	@echo "  make build         - 构建所有平台"
	@echo "  make build-current - 构建当前平台"
	@echo "  make test          - 运行测试"
	@echo "  make clean         - 清理构建文件"
	@echo "  make deps          - 下载依赖"
	@echo "  make run           - 构建并运行程序"
	@echo "  make dev           - 开发模式运行"
	@echo "  make lint          - 代码检查和格式化"
	@echo "  make install       - 安装到系统"
	@echo "  make uninstall     - 从系统卸载"
	@echo "  make info          - 显示构建信息"
	@echo "  make help          - 显示此帮助"
	@echo ""
	@echo "示例:"
	@echo "  make build VERSION=v1.1.0    - 构建版本 v1.1.0"
	@echo "  make build-linux/amd64       - 只构建 Linux 64位版本"