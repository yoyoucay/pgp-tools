# PGP Tools

Desktop app to encrypt and decrypt files with OpenPGP keys. One portable program per OS, no GnuPG required, no admin or root rights needed.

- Windows 10/11, Linux (X11 + Wayland), macOS
- Reads armored (`.asc`) and binary (`.gpg`/`.pgp`) keys and messages, including files saved by Windows Notepad (CRLF, BOM)
- Interoperable with GnuPG in both directions
- Checks every field as you type and says what to fix, in plain language

## Quick Start

### Windows

1. Download `pgp-tools-windows-amd64.exe` from the [Releases](https://github.com/yoyoucay/pgp-tools/releases) page. Optional: compare its hash with `SHA256SUMS.txt` (`Get-FileHash pgp-tools-windows-amd64.exe`).
2. Double-click it. It runs as-is (portable): no installer, no admin prompt.
3. Optional: **Tools > Install to Start Menu...** copies it to your user profile and adds a Start Menu shortcut.

SmartScreen may warn on first launch because the exe is not code-signed: **More info > Run anyway**.

### Linux

```bash
chmod +x pgp-tools-linux-amd64
./pgp-tools-linux-amd64              # portable
./pgp-tools-linux-amd64 --install    # or Tools > Install to app menu...
```

### macOS

`chmod +x pgp-tools-macos-arm64`, then run it. First launch may need right-click > Open (unsigned binary). No install step.

### Per-user install locations (no admin/root)

| | Windows | Linux/BSD |
|---|---|---|
| Program | `%LOCALAPPDATA%\Programs\PGP Tools\pgp-tools.exe` | `$XDG_BIN_HOME` or `~/.local/bin/pgp-tools` |
| Menu entry | `%APPDATA%\Microsoft\Windows\Start Menu\Programs\PGP Tools.lnk` | `~/.local/share/applications/pgp-tools.desktop` |
| Icon | inside the exe | `~/.local/share/icons/hicolor/scalable/apps/pgp-tools.svg` |
| Settings | `%APPDATA%\fyne\io.github.yoyoucay.pgptools\` | `~/.config/fyne/io.github.yoyoucay.pgptools/` |

Remove with **Tools > Remove from this computer...** or `--uninstall`. Keys and encrypted files are never touched.

If the Start Menu shortcut cannot be created (PowerShell blocked by company policy), the program is still installed and the app says so; pin it manually.

## Build from source

Requires Go (see `go.mod`) and a C compiler (Fyne uses cgo for OpenGL).

### Windows (native)

Install Go and MinGW-w64 gcc (e.g. MSYS2: `pacman -S mingw-w64-ucrt-x86_64-gcc`, then add its `bin` to `PATH`).

```powershell
powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1          # tests + dist\pgp-tools.exe
powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version v1.0.0 -SkipTests
```

`windres` (part of MinGW) embeds the icon, version info and a manifest: `asInvoker` (never asks for admin), per-monitor DPI awareness (sharp on scaled displays), long-path support. Without `windres` the exe still builds, without those.

### Linux (and cross-building the Windows exe)

```bash
# Debian/Ubuntu GUI headers
sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols

make check      # gofmt, vet, race tests, build -> bin/pgp-tools
make report     # unit test report -> reports/index.html (see "Unit test report")
make install    # per-user install, no sudo

# Windows exe from Linux: sudo apt-get install gcc-mingw-w64-x86-64
make windows VERSION=v1.0.0   # -> dist/pgp-tools.exe
```

X11-only build (drops the libwayland-client link): `go build -tags x11 -o bin/pgp-tools ./cmd/pgp-tools`.

## Usage

Two separate screens, reachable from the left tabs or the **Tools** menu. **Help > Which key goes where?** shows the key roles.

### Getting files in

- **Browse** next to each field, or type/paste a path. Quotes from Windows **Copy as path** are removed, `/` and `\` both work, `~` means your home folder.
- **Drag and drop** files onto the window. The app detects what each file is:

| Dropped file | Goes to |
|---|---|
| Encrypted file (`.gpg`, `.pgp`, `.asc` message) | Decrypt > Input file (switches tab) |
| Any other file | Encrypt > Input file (switches tab) |
| Public key | Public key field of the current tab |
| Private key | Private key field of the current tab |

### Encrypt

| Field | Required | Purpose |
|---|---|---|
| Input file | yes | File to protect |
| Public key | yes | Recipient's public key. Only the matching private key can decrypt |
| Private key | no | Your private key. Adds your signature |
| Passphrase | if private key is protected | Unlocks your private key for signing. Disabled until needed |
| Output file | yes | Defaults to `<input>.gpg`, or `<input>.asc` with "Save as text" |

### Decrypt

| Field | Required | Purpose |
|---|---|---|
| Input file | yes | `.gpg`, `.pgp` or `.asc` message |
| Private key | yes | Your private key the message was encrypted to |
| Passphrase | if private key is protected | Unlocks your private key. Disabled until needed |
| Public key | no | Sender's public key. When set, decryption fails unless the message carries a valid signature from this key |
| Output file | yes | Defaults to input without `.gpg`/`.pgp`/`.asc`, else `<input>.decrypted` |

### Why the key roles are this way

OpenPGP is asymmetric: anyone can encrypt with a **public** key; only the **private** key holder can decrypt. A private key used during encryption only signs; a public key used during decryption only verifies that signature. Encrypting "with a private key" would give no confidentiality, since the public key that reverses it is public.

## Validation

Every field is checked about a quarter second after you stop typing, in the background. A line under each field shows the result:

| Look | Meaning |
|---|---|
| Plain text | Hint for an empty or optional field |
| Green, check mark | Field is fine; shows what was found (size, key owner, key ID) |
| Yellow, warning sign | Usable, but read this (e.g. output will be replaced) |
| Red, error sign | Must be fixed before running |

Above the run button, one line lists what is still missing, e.g. `To continue: choose your private key; enter the passphrase.` The **Encrypt** / **Decrypt** button is enabled only when nothing is missing.

Checks performed before you press the button:

| Field | Checked |
|---|---|
| Input file | Exists, is a file not a folder, readable; Decrypt: is an OpenPGP encrypted file, not password-only |
| Public key | Is an OpenPGP key; Encrypt: can encrypt (not expired, revoked or sign-only); warns if the file also contains a private key |
| Private key | Contains a private key, not just a public one; Encrypt: can sign; Decrypt: **matches the file's recipients** |
| Passphrase | Enabled only for protected keys; verified against the key as you type (`Incorrect passphrase. Check Caps Lock and keyboard layout.`) |
| Output file | Folder exists, not a folder itself, differs from input, Windows-invalid characters (`< > : " \| ? *`), warns if it will be replaced |

The same rules run again when you press the button; nothing relies on the live check alone.

## Behavior and edge cases

| Situation | Result |
|---|---|
| Output file exists | Confirm dialog before replacing |
| Output path equals input path | Blocked by validation |
| Failure or Cancel mid-run | No output file left behind (written to hidden temp file, renamed on success) |
| Output file open in another program (Windows) | `Access denied. If the file is open in another program, close it and try again.` |
| Tampered or truncated ciphertext | `message integrity check failed`; no plaintext written |
| Message not encrypted to given private key | Blocked by validation; shows the key IDs the file is for |
| Hidden recipient (anonymous key ID) | Allowed; decryption tries the key |
| Public key given on Decrypt, message unsigned | Rejected: `message is not signed but signature verification was requested` |
| Public key given on Decrypt, signed by another key | Rejected: `signed by a key other than the provided public key` |
| No public key on Decrypt, message signed | Decrypts; reports signature present but NOT verified |
| Passphrase-only (symmetric) message | Blocked as unsupported |
| Key file > 16 MiB or not a key | Rejected before reading input |
| Key or message saved with CRLF and BOM (Notepad) | Accepted |
| Running copy replaced during install/upgrade (Windows) | Old exe is moved aside, new one installed |

Decrypted output is created with mode `0600` on Linux/macOS (owner-only).

## Data stored

Only key file paths and the "Save as text" checkbox, in Fyne preferences (paths above). Passphrases are never stored; a correct-passphrase check is cached in memory only as a hash for the running session. No network access.

## CLI flags

| Flag | Effect |
|---|---|
| (none) | Open the GUI |
| `--install` | Per-user install: Windows Start Menu or Linux/BSD app menu. Exit 2 on macOS; exit 1 if only the menu entry failed |
| `--uninstall` | Remove per-user install |
| `--version` | Print version |

On Windows the release exe is a GUI program; run flags from `cmd` or PowerShell and the output appears in that window.

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

## Releasing

| Trigger | Builds | Publishes |
|---|---|---|
| Pull request, push to `main` | Linux: tests, unit test report, binary | CI artifacts |
| Manual run (Actions > build > Run workflow) | Linux, Windows, macOS | CI artifacts |
| Tag `v*` | Linux, Windows, macOS | GitHub Release with all binaries and `SHA256SUMS.txt` |

Windows and macOS run only for tags and manual runs because private repos bill their Actions minutes at 2x and 10x.

```bash
git checkout main && git pull
git tag v0.1.0            # a tag with a hyphen (v0.2.0-rc1) becomes a pre-release
git push origin v0.1.0
```

The release is created only if all three builds pass.

## Development

See `AGENTS.md` (contributor/agent workflow), `docs/ARCHITECTURE.md`, and run `./init.sh` before and after changes.
