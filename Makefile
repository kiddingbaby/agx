GO ?= go
BIN ?= .build/agx
CMD ?= ./cmd/agx

.PHONY: help build rebuild run test smoke fmt vet tidy clean install

help:
	@echo "Targets:"
	@echo "  make build    - Build $(BIN)"
	@echo "  make rebuild  - Clean and build"
	@echo "  make run      - Build and run $(BIN)"
	@echo "  make test     - Run all Go tests"
	@echo "  make smoke    - Run integration smoke script"
	@echo "  make fmt      - Format all Go packages"
	@echo "  make vet      - Run go vet"
	@echo "  make tidy     - Run go mod tidy"
	@echo "  make clean    - Remove built binary"
	@echo "  make install  - Install to GOPATH/bin"

build:
	@mkdir -p "$(dir $(BIN))"
	$(GO) build -o $(BIN) $(CMD)

rebuild: clean build

run: build
	$(BIN)

test:
	$(GO) test ./...

smoke:
	bash tests/integration/smoke-go.sh

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BIN)

install:
	$(GO) install $(CMD)
