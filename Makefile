APP     := pgp-tools
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin/$(APP)

.PHONY: build test vet check install uninstall clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/$(APP)

test:
	go test -race -count=1 ./internal/...

vet:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt needed"; exit 1; }
	go vet ./...

check: vet test build

# Per-user install, no root: ~/.local/bin + desktop entry + icon.
install: build
	./$(BIN) --install

uninstall:
	./$(BIN) --uninstall

clean:
	go clean
	rm -f $(BIN)
