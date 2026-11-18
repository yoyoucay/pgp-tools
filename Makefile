APP     := pgp-tools
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin/$(APP)
REPORTS := reports
PKGS    := ./internal/... ./tools/...

.PHONY: build test vet check report install uninstall clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/$(APP)

test:
	go test -race -count=1 $(PKGS)

# Unit test report: reports/index.html, junit.xml, summary.md, coverage.html.
# Reports are written even when tests fail; the target then exits non-zero.
report:
	@mkdir -p $(REPORTS)
	-go test -race -count=1 -json -covermode=atomic -coverprofile=$(REPORTS)/coverage.out $(PKGS) > $(REPORTS)/test.json
	@if [ -s $(REPORTS)/coverage.out ]; then go tool cover -html=$(REPORTS)/coverage.out -o $(REPORTS)/coverage.html; fi
	go run ./tools/testreport -in $(REPORTS)/test.json -coverprofile $(REPORTS)/coverage.out \
		-junit $(REPORTS)/junit.xml -md $(REPORTS)/summary.md -html $(REPORTS)/index.html \
		-title "PGP Tools unit tests" -coverage-link coverage.html -fail

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
	rm -f $(REPORTS)/test.json $(REPORTS)/coverage.out $(REPORTS)/coverage.html $(REPORTS)/junit.xml $(REPORTS)/summary.md $(REPORTS)/index.html
