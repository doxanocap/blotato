LOCAL_BIN := $(CURDIR)/_bin
SWAGGO_VERSION := v1.16.6
SWAGGO_BIN ?= $(LOCAL_BIN)/swag
SWAGGO_DIRS := ./cmd/api,./internal/io/api/controllers/workflows,./internal/io/api/controllers/models,./internal/domain/errs,./internal/domain/entity/types

.PHONY: install go-mod-download start build test docs

install: go-mod-download $(SWAGGO_BIN)

go-mod-download:
	go mod download

$(LOCAL_BIN):
	mkdir -p $(LOCAL_BIN)

$(SWAGGO_BIN): | $(LOCAL_BIN)
	GOBIN=$(LOCAL_BIN) go install github.com/swaggo/swag/cmd/swag@$(SWAGGO_VERSION)

start:
	go run ./cmd/api

build: | $(LOCAL_BIN)
	go build -o $(LOCAL_BIN)/blotato ./cmd/api

test:
	go test -race ./...

docs: $(SWAGGO_BIN)
	$(SWAGGO_BIN) fmt -d $(SWAGGO_DIRS) -g main.go
	$(SWAGGO_BIN) init --parseInternal -d $(SWAGGO_DIRS) -g main.go -o ./docs
