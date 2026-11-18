#!/usr/bin/env bash
# Startup + verification gate. Run before editing and before claiming done.
set -euo pipefail
cd "$(dirname "$0")"

echo "=== toolchain ==="
command -v go >/dev/null || { echo "go not found; install Go >= $(awk '/^go /{print $2}' go.mod)"; exit 1; }
go version

if [ "$(uname -s)" = "Linux" ]; then
  missing=()
  for pc in gl x11 xcursor xrandr xinerama xi wayland-client xkbcommon; do
    pkg-config --exists "$pc" 2>/dev/null || missing+=("$pc")
  done
  if [ ${#missing[@]} -gt 0 ]; then
    echo "missing GUI dev libs: ${missing[*]}"
    echo "Debian/Ubuntu: sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols"
    echo "(crypto core still testable: go test ./internal/pgp ./internal/install)"
    exit 1
  fi
fi

echo "=== deps ==="
go mod download

echo "=== make check (gofmt, vet, race tests, build) ==="
make check

echo "=== OK: bin/pgp-tools built ==="
echo "Next: read feature_list.json, pick ONE feature, verify with ./init.sh before marking done."
