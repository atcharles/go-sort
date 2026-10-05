.DEFAULT_GOAL := help

GO_FILES := $(shell find . -type d \( -name .git -o -name .agent-bus -o -name .codex -o -name vendor -o -name bin -o -name testdata \) -prune -o -type f -name '*.go' -print)
GO_TOOL := go tool -modfile=tools/go.mod

.PHONY: help build run fmt fmt-check lint style-check test vuln ci deps-check deps-upgrade clean

help:
	@printf '%s\n' 'GO-SORT 开发命令：' '  build         构建 bin/go-sort' '  run           运行（通过 ARGS 传参）' '  fmt           go-sort → gofmt → goimports' '  fmt-check     在临时副本检查格式无差异' '  lint          共享规则与严格 shadow 检查' '  style-check   私有 kit noelse 检查' '  test          单元与黄金测试（-race）' '  vuln          主模块漏洞检查' '  ci            顺序执行全部检查' '  deps-check    列出依赖升级' '  deps-upgrade  升级依赖并执行 ci' '  clean         清理本项目构建产物'

build:
	@mkdir -p bin
	go build -trimpath -o bin/go-sort .

run:
	go run . $(ARGS)

fmt:
	go run . -tests .
	gofmt -w $(GO_FILES)
	$(GO_TOOL) goimports -w $(GO_FILES)

fmt-check:
	@sh scripts/fmt-check.sh

lint:
	$(GO_TOOL) golangci-lint run --config .golangci.yml ./...

style-check:
	$(GO_TOOL) noelse .

test:
	go test -race ./...

vuln:
	$(GO_TOOL) govulncheck ./...

ci:
	$(MAKE) fmt-check
	$(MAKE) lint
	$(MAKE) style-check
	$(MAKE) vuln
	$(MAKE) test

deps-check:
	go list -m -u all
	go list -modfile=tools/go.mod -m -u all

deps-upgrade:
	go get -u ./...
	go mod tidy
	cd tools && go get -u tool && go get github.com/gobwas/glob@v0.2.3 github.com/nishanths/exhaustive@v0.13.0 github.com/nishanths/predeclared@v0.2.2 && go mod tidy
	$(MAKE) ci

clean:
	rm -rf bin
