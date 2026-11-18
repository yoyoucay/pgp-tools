# PGP Tools

Desktop app to encrypt and decrypt files with OpenPGP keys. Single portable binary, no GnuPG required, no root needed to install.

- Linux (X11 + Wayland), Windows, macOS
- Reads armored (`.asc`) and binary (`.gpg`/`.pgp`) keys and messages
- Interoperable with GnuPG in both directions

## Quick Start

### Portable (no install)

Download the binary for your OS from the latest CI run or release, then:

```bash
chmod +x pgp-tools-linux-amd64
./pgp-tools-linux-amd64
```

Windows: double-click `pgp-tools-windows-amd64.exe`. macOS: `chmod +x` then run; first launch may need right-click > Open (unsigned binary).

### Install for current user (Linux/BSD, no root)

```bash
./pgp-tools-linux-amd64 --install     # copies itself to ~/.local/bin, adds app menu entry + icon
pgp-tools --uninstall                 # removes those three files
```

| Installed file | Location |
|---|---|
| Binary | `$XDG_BIN_HOME` or `~/.local/bin/pgp-tools` |
| Menu entry | `$XDG_DATA_HOME` or `~/.local/share/applications/pgp-tools.desktop` |
| Icon | `~/.local/share/icons/hicolor/scalable/apps/pgp-tools.svg` |

If `~/.local/bin` is not in `PATH` the menu entry still works; the installer prints a note.

### Build from source

Requires Go (see `go.mod`) and a C compiler (Fyne uses cgo for OpenGL).

```bash
# Debian/Ubuntu GUI headers
sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols

make check      # gofmt, vet, race tests, build -> bin/pgp-tools
make report     # unit test report -> reports/index.html (see "Unit test report")
make install    # per-user install, no sudo
```

X11-only build (drops the libwayland-client link): `go build -tags x11 -o bin/pgp-tools ./cmd/pgp-tools`.

## Usage

Two separate screens, reachable from the left tabs or the **Tools** menu.

### Encrypt

| Field | Required | Purpose |
|---|---|---|
| Input file | yes | File to protect |
| Public key | yes | Recipient's public key. Only the matching private key can decrypt |
| Private key | no | Your private key. Adds your signature |
| Passphrase | if private key is protected | Unlocks your private key for signing |
| Output file | yes | Defaults to `<input>.gpg`, or `<input>.asc` with ASCII armor |

### Decrypt

| Field | Required | Purpose |
|---|---|---|
| Input file | yes | `.gpg`, `.pgp` or `.asc` message |
| Private key | yes | Your private key the message was encrypted to |
| Passphrase | if private key is protected | Unlocks your private key |
| Public key | no | Sender's public key. When set, decryption fails unless the message carries a valid signature from this key |
| Output file | yes | Defaults to input without `.gpg`/`.pgp`/`.asc`, else `<input>.decrypted` |

### Why the key roles are this way

OpenPGP is asymmetric: anyone can encrypt with a **public** key; only the **private** key holder can decrypt. A private key used during encryption only signs; a public key used during decryption only verifies that signature. Encrypting "with a private key" would give no confidentiality, since the public key that reverses it is public.

## Behavior and edge cases

| Situation | Result |
|---|---|
| Output file exists | Confirm dialog before replacing |
| Output path equals input path | Rejected |
| Failure or Cancel mid-run | No output file left behind (written to hidden temp file, renamed on success) |
| Tampered or truncated ciphertext | `message integrity check failed`; no plaintext written |
| Wrong passphrase | `passphrase is incorrect` |
| Protected key, empty passphrase | `private key is protected; passphrase required` |
| Message not encrypted to given private key | `message is not encrypted for this private key` |
| Public key given on Decrypt, message unsigned | Rejected: `message is not signed but signature verification was requested` |
| Public key given on Decrypt, signed by another key | Rejected: `signed by a key other than the provided public key` |
| No public key on Decrypt, message signed | Decrypts; reports signature present but NOT verified |
| Passphrase-only (symmetric) message | Rejected as unsupported |
| Expired, revoked or sign-only recipient key | `public key has no valid encryption subkey` |
| Key file > 16 MiB or not a key | Rejected before reading input |
| Directory or device as input | Rejected |

Decrypted output is created with mode `0600`.

## Data stored

Only key file paths and the armor checkbox, in Fyne preferences (`~/.config/fyne/io.github.yoyoucay.pgptools/` on Linux). Passphrases are never stored. No network access.

## CLI flags

| Flag | Effect |
|---|---|
| (none) | Open the GUI |
| `--install` | Per-user install (Linux/BSD). Exit 2 on other OS |
| `--uninstall` | Remove per-user install |
| `--version` | Print version |

## Unit test report

```bash
make report
xdg-open reports/index.html     # macOS: open, Windows: start
```

Runs every unit test with the race detector and coverage, then renders the results with `tools/testreport` (standard library only, no extra install).

| File in `reports/` | Use |
|---|---|
| `index.html` | Standalone page: totals, per-package status and coverage, failure output, every test with duration and skip reason |
| `coverage.html` | Line-by-line coverage from `go tool cover` (linked from `index.html`) |
| `junit.xml` | JUnit XML for CI dashboards (Jenkins, GitLab, Azure DevOps, test-reporter actions) |
| `summary.md` | Markdown summary; CI appends it to the GitHub job summary |
| `test.json`, `coverage.out` | Raw `go test -json` stream and coverage profile |

`make report` writes all reports even when tests fail, then exits non-zero. In CI the Linux job publishes `summary.md` to the run page and uploads `reports/` as the `unit-test-report` artifact, including on failure.

### What the report covers

| Package | Tests |
|---|---|
| `internal/pgp` | Encrypt/decrypt round trips (armored, binary, signed, verified), wrong passphrase, missing passphrase, wrong key type, wrong recipient, signer mismatch, unsigned-but-verify, tampered ciphertext, empty and plaintext input, passphrase-only messages, cancel with no leftover temp file, default output names, GnuPG interop both directions (skipped when `gpg` is absent) |
| `internal/install` | Install/uninstall in a temp home, re-install from the installed copy, Exec quoting for paths with spaces or `$`, XDG overrides |
| `internal/ui` | Error-to-message mapping, byte formatting, signature status text. Widget layout is checked manually, not unit-tested |
| `tools/testreport` | Parsing of build failures, crashed tests, skips, subtests and stray lines; coverage merge; JUnit validity; Markdown escaping; HTML escaping of test output; CLI exit codes |

### Running the tool directly

```bash
go test -json -coverprofile=cover.out ./... > test.json
go run ./tools/testreport -in test.json -coverprofile cover.out \
  -html report.html -junit junit.xml -md -
```

| Flag | Default | Meaning |
|---|---|---|
| `-in` | `-` (stdin) | `go test -json` output |
| `-coverprofile` | none | Coverage profile; a missing file is ignored (build failed before tests ran) |
| `-html`, `-junit`, `-md` | none | Output paths; at least one is required. `-md -` prints to stdout |
| `-title` | `Unit test report` | HTML page title |
| `-coverage-link` | none | Relative link from the HTML report to a coverage page |
| `-fail` | off | Exit 1 when any test or package failed |

| Exit code | Meaning |
|---|---|
| 0 | Reports written (with `-fail`: and all tests passed) |
| 1 | `-fail` set and a test, package or build failed |
| 2 | Bad flags, no output selected, or input has no `go test -json` events |
| 3 | Input, coverage or output file I/O error |

Edge cases: a test with no pass/fail event (panic, timeout, killed binary) is reported as failed with a note. Compile errors appear as `BUILD FAILED` with the compiler output. Lines that are not JSON are kept in a "Non-JSON output" section. Test time is the sum per package; packages run in parallel, so it can exceed wall-clock time.

## Development

See `AGENTS.md` (contributor/agent workflow), `docs/ARCHITECTURE.md`, and run `./init.sh` before and after changes.
