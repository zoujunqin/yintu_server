# Spring Slumber Server - Makefile
#
# 常用目标：
#   make swag     重新生成 Swagger / OpenAPI 文档（基于 handler 上的 swag 注解）
#   make build    编译二进制到 bin/server
#   make run      直接 go run（默认读取 .env）
#   make test     跑单测
#   make tidy     go mod tidy
#   make install-tools  本地安装 swag CLI（如尚未安装）
#
# 设计原则：Makefile 是 npm scripts 之外的兜底入口；CI / Docker 仍可直接调用对应命令。

# swag CLI 路径：优先用本地 GOPATH/bin，缺失时退回 PATH。
SWAG     ?= $(shell go env GOPATH)/bin/swag
ifeq (,$(wildcard $(SWAG)))
SWAG := swag
endif

# 默认端口（与 .env.example 一致）。
PORT ?= 8080

.PHONY: help swag build run test tidy install-tools clean

help: ## 打印可用目标
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

install-tools: ## 安装 swag CLI 到 $(go env GOPATH)/bin
	@command -v $(SWAG) >/dev/null 2>&1 || go install github.com/swaggo/swag/cmd/swag@v1.16.6

swag: install-tools ## 从 handler 注解重新生成 internal/docs
	$(SWAG) init -g cmd/server/main.go -o internal/docs --parseDependency --parseInternal

build: ## 编译二进制到 bin/server
	@mkdir -p bin
	go build -o bin/server ./cmd/server

run: ## 本地运行（读取 .env）
	APP_ENV=$${APP_ENV:-development} HTTP_PORT=$(PORT) go run ./cmd/server

test: ## 跑单测
	go test ./...

tidy: ## go mod tidy
	go mod tidy

keygen: ## 生成 RSA-2048 keypair 并追加到 .env（用 -e 指定其他文件）
	@go run ./cmd/keygen -out .env
	@echo "tip: 把 SIGN_PUBLIC_KEY 同步到前端 NEXT_PUBLIC_RSA_PUBLIC_KEY"

keygen-prod: ## 生成 RSA-2048 keypair 到 .env.production
	@go run ./cmd/keygen -out .env.production -bits 2048
	@echo "tip: 把 SIGN_PUBLIC_KEY 同步到前端 NEXT_PUBLIC_RSA_PUBLIC_KEY"

clean: ## 删除构建产物
	rm -rf bin