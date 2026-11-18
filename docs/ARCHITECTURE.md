# Architecture

```
cmd/pgp-tools        flags -> install | uninstall | ui.Run
internal/ui          Fyne tabs (Encrypt, Decrypt), runner (busy/cancel/errors)
internal/pgp         Encrypt(ctx, EncryptRequest) / Decrypt(ctx, DecryptRequest)
internal/install     XDG per-user install
assets               embedded icon.svg + pgp-tools.desktop template
tools/testreport     dev tool: go test -json -> HTML / JUnit / Markdown
```

Dependency direction: `cmd -> ui -> pgp`, `cmd -> install`. `pgp` and `install` import no UI code and are fully unit-tested without a display.

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
