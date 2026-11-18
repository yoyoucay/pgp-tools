# Architecture

```
cmd/pgp-tools        flags -> install | uninstall | ui.Run; Windows console attach
internal/ui          Fyne tabs (Encrypt, Decrypt), status lines, runner (busy/cancel/errors), drop handler
internal/formcheck   form -> per-field Result + Ready + Summary; drop routing
internal/pgp         Encrypt / Decrypt; InspectKey / InspectMessage / CheckPassphrase / Classify
internal/install     per-user install: XDG (Linux/BSD), Start Menu (Windows)
winres               Windows icon, manifest, version resource -> .syso
assets               embedded icon.svg + pgp-tools.desktop template
tools/testreport     dev tool: go test -json -> HTML / JUnit / Markdown
```

Dependency direction: `cmd -> ui -> formcheck -> pgp`, `ui -> install`, `cmd -> install`. `pgp` and `install` import no UI code and are fully unit-tested without a display.

## Crypto flow

**Encrypt**
1. `Validate()` request.
2. Load recipient keyring; keep entities with a valid encryption subkey.
3. If signer set: load private keyring, unlock with passphrase, pick first entity with a usable signing key.
4. Stream input -> (optional armor) -> `openpgp.Encrypt` (AES-256) -> temp file -> rename.

**Decrypt**
1. Load and unlock private keyring; append verify keyring if given.
2. Sniff first 1 KiB for armor; decode if present.
3. `openpgp.ReadMessage`; prompt callback only fires when no key matched and distinguishes symmetric-only messages.
4. Stream body to temp file. MDC/AEAD integrity is checked at EOF.
5. Signature policy (`checkSignature`), then rename temp file to target.

Both paths wrap the input reader with a context check so Cancel stops within one buffer read.

## Validation flow

Each field change calls `liveCheck.schedule()`: the form is read on the UI thread, a 250 ms timer restarts, then `formcheck.Checker` runs off the UI thread and the result is applied via `fyne.Do` only if no newer input arrived (generation counter). Checks read file metadata, parse key files (max 16 MiB) and read message headers up to the encrypted body, never the body itself. Passphrase checks run the key's KDF, so results are cached by a SHA-256 of (path, size, mtime, passphrase).

## Windows specifics

| Concern | Handling |
|---|---|
| No admin rights | Manifest `asInvoker`; install to `%LOCALAPPDATA%\Programs`, shortcut in the user's Start Menu |
| Shortcut creation | PowerShell + `WScript.Shell`; paths passed via environment variables; result verified on disk |
| Running exe replaced or removed | Renamed aside first (Windows allows renaming a running exe, not deleting it) |
| GUI exe and CLI flags | `AttachConsole(ATTACH_PARENT_PROCESS)` so `--install` output reaches the terminal |
| Paths | `filepath.FromSlash` on dialog/drop URIs; quotes from "Copy as path" stripped; case-insensitive compare |
| Notepad files | UTF-8 BOM stripped before armor parsing; CRLF accepted |
| Locked output file | `fs.ErrPermission` mapped to "close it in the other program" message |

## Errors

`internal/pgp/errors.go` defines sentinels (`ErrWrongPassphrase`, `ErrNoMatchingKey`, `ErrIntegrity`, ...) plus `ValidationError` and `KeyError`. Library errors are mapped to these at the boundary. `internal/ui/messages.go` turns them into one-line dialog text.

## Threading

Crypto runs in a goroutine per job. All widget updates from it go through `fyne.Do`. Inputs are disabled while a job runs; one job per tab.

## Portability

| Concern | Choice |
|---|---|
| OpenPGP | `github.com/ProtonMail/go-crypto` (pure Go, maintained fork of x/crypto/openpgp) |
| GUI | Fyne v2 (OpenGL via cgo); one binary per OS |
| Install | `--install` copies the running binary; no package manager, no root |
| Settings | Fyne preferences, per user |

## Test reporting

`make report` pipes `go test -json` into `tools/testreport`, which builds one in-memory model (packages, tests, coverage) and renders three views from it: `html/template` page (auto-escaped output), `encoding/xml` JUnit, and Markdown for the GitHub job summary. Build failures (`build-fail` events, Go 1.24+), tests with no terminal event, and package failures without a failing test are all surfaced as failures so no output format can report green by omission.
