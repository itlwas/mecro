.PHONY: runtime build generate build-quick
NAME = mecro
VERSION = 2.0.14
DATE = $(shell date +"%d.%m.%Y")
GOBIN ?= $(shell go env GOPATH)/bin
GOVARS = -X github.com/zyedidia/micro/v2/internal/util.Version=$(VERSION) -X 'github.com/zyedidia/micro/v2/internal/util.CompileDate=$(DATE)'
DEBUGVAR = -X github.com/zyedidia/micro/v2/internal/util.Debug=ON
CGO_ENABLED := $(if $(CGO_ENABLED),$(CGO_ENABLED),0)
ADDITIONAL_GO_LINKER_FLAGS := ""
GOHOSTOS = $(shell go env GOHOSTOS)
OPTIMIZATION_FLAGS = -trimpath -ldflags "-s -w -compressdwarf=true -buildid= $(GOVARS) $(ADDITIONAL_GO_LINKER_FLAGS)"
ifeq ($(GOOS),android)
    CGO_ENABLED = 1
endif
all: build
	@echo
	@file ./$(NAME)
update:
	@git remote add upstream https://github.com/zyedidia/micro 2>/dev/null || true
	git pull --rebase upstream master
upgrade: update
	go get -u ./...
build: generate build-quick clean-hdr-files
build-quick:
	CGO_ENABLED=$(CGO_ENABLED) go build $(OPTIMIZATION_FLAGS) -gcflags=all="-l -B" ./cmd/$(NAME)
build-dbg:
	CGO_ENABLED=$(CGO_ENABLED) go build -trimpath -ldflags "$(ADDITIONAL_GO_LINKER_FLAGS) $(DEBUGVAR)" ./cmd/$(NAME)
build-tags: fetch-tags generate
	CGO_ENABLED=$(CGO_ENABLED) go build $(OPTIMIZATION_FLAGS) ./cmd/$(NAME)
build-all: build
install: generate
	go install $(OPTIMIZATION_FLAGS) ./cmd/$(NAME)
	@mkdir -p ~/.local/share/applications/
	cp -f ./runtime/$(NAME).desktop ~/.local/share/applications/
install-all: install
fetch-tags:
	git fetch --tags --force
generate:
	GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) go generate ./runtime
test:
	go test ./internal/...
	go test ./cmd/...
bench:
	for i in 1 2 3; do \
		go test -bench=. ./internal/...; \
	done > benchmark_results
	benchstat benchmark_results
bench-baseline:
	for i in 1 2 3; do \
		go test -bench=. ./internal/...; \
	done > benchmark_results_baseline
bench-compare:
	for i in 1 2 3; do \
		go test -bench=. ./internal/...; \
	done > benchmark_results
	benchstat -alpha 0.15 benchmark_results_baseline benchmark_results
clean:
	rm -f ./$(NAME)
clean-hdr-files:
	rm -f ./runtime/syntax/*.hdr