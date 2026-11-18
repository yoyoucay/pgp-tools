APP     := pgp-tools
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin/$(APP)
REPORTS := reports
PKGS    := ./internal/... ./tools/...

# Windows cross-build (Debian/Ubuntu: apt-get install gcc-mingw-w64-x86-64).
WIN_CC  ?= x86_64-w64-mingw32-gcc
WINDRES ?= x86_64-w64-mingw32-windres
WIN_EXE := dist/$(APP).exe
SYSO    := cmd/$(APP)/rsrc_windows_amd64.syso
# Numeric x.y.z for the exe version resource; 0.0.0 when VERSION is not a tag.
VER_NUM := $(or $(shell echo "$(VERSION)" | sed -nE 's/^v?([0-9]+)\.([0-9]+)\.([0-9]+).*/\1 \2 \3/p'),0 0 0)

.PHONY: build test vet check report windows install uninstall clean

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

# Portable Windows exe with icon, manifest (no admin prompt, DPI aware) and version info.
windows:
	cd winres && $(WINDRES) -DVER_MAJOR=$(word 1,$(VER_NUM)) -DVER_MINOR=$(word 2,$(VER_NUM)) -DVER_PATCH=$(word 3,$(VER_NUM)) \
		-O coff -i pgp-tools.rc -o ../$(SYSO)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(WIN_CC) \
		go build -trimpath -ldflags "$(LDFLAGS) -H windowsgui" -o $(WIN_EXE) ./cmd/$(APP)

# Per-user install, no root: ~/.local/bin + desktop entry + icon.
install: build
	./$(BIN) --install

uninstall:
	./$(BIN) --uninstall

clean:
	go clean
	rm -f $(BIN) $(WIN_EXE) $(SYSO)
	rm -f $(REPORTS)/test.json $(REPORTS)/coverage.out $(REPORTS)/coverage.html $(REPORTS)/junit.xml $(REPORTS)/summary.md $(REPORTS)/index.html
