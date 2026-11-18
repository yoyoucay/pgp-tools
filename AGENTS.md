# AGENTS.md

PGP Tools: portable desktop app (Go + Fyne) to encrypt/decrypt files with OpenPGP keys. Pure-Go crypto (`github.com/ProtonMail/go-crypto`), no GnuPG dependency, single binary per OS (Windows, Linux, macOS), per-user install without admin/root.

## Startup Workflow

1. Read this file, then `README.md` and `docs/ARCHITECTURE.md`.
2. Run `./init.sh`. If it fails, fix the baseline before new scope.
3. Read `feature_list.json` and `progress.md`; `git log --oneline -5`.

## Layout

| Path | Owns |
|---|---|
| `internal/pgp` | Crypto core + `Inspect*`/`Classify` (read headers without decrypting). No UI imports. |
| `internal/formcheck` | Live field validation and drag-and-drop routing. No Fyne imports; all rules unit-tested. |
| `internal/install` | Per-user install. `install_unix.go` (XDG), `install_windows.go` (Start Menu `.lnk` via PowerShell). |
| `internal/ui` | Fyne widgets only. Calls `formcheck` for validation, `pgp` to run; no rules or crypto here. |
| `cmd/pgp-tools` | Flags (`--install`, `--uninstall`, `--version`) and UI launch. |
| `assets` | Embedded icon + desktop entry template. |
| `winres`, `scripts/build-windows.ps1` | Windows exe resources (icon, asInvoker/DPI manifest, version) and native build. |
| `tools/testreport` | `go test -json` to HTML/JUnit/Markdown report. Standard library only; not shipped in the app binary. |

## Invariants (do not break)

- Key roles: encrypt = recipient PUBLIC key (+ optional own private key/passphrase to sign); decrypt = own PRIVATE key + passphrase (+ optional sender public key to verify). Do not swap.
- Output is written to a temp file and renamed only after integrity/signature checks pass. No partial plaintext on failure or cancel.
- When a verify key is given, decrypt fails unless signed by that exact key.
- Passphrases are never persisted, logged, or shown. Only key file paths go to preferences.
- Nothing requires root. Install targets come from `XDG_*` or `~/.local`.
- UI updates from goroutines go through `fyne.Do`.
- New error cases: add a sentinel in `internal/pgp/errors.go`, map in `internal/ui/messages.go`, test in `crypto_test.go`.
- New field rules go in `internal/formcheck` with a test; `pgp.*Request.Validate()` stays as the backstop at run time.
- Windows: no admin prompt (manifest `asInvoker`), no hardcoded `/` paths, compare paths case-insensitively, PowerShell inputs via env vars only.

## Working Rules

- One feature at a time from `feature_list.json`; mark `in-progress` while working.
- Do not edit files outside the active feature's scope.
- Do not add a GnuPG/cgo crypto dependency.
- Commit author: `yoyoucay`. No AI co-author or trailer lines.
- Branches: each new change gets exactly one new branch from the latest `main` (e.g. `feat/<topic>`, `fix/<topic>`, `docs/<topic>`) and its own PR. Never commit new work to `main` or to another change's branch. Fixes requested on an open PR go on that PR's branch.

## Definition of Done

- Behavior implemented and covered by a test (or a recorded manual GUI check for UI-only changes).
- `./init.sh` passes; `make report` shows no failures and no coverage drop in touched packages.
- `feature_list.json` status + evidence updated; `progress.md` updated.
- README updated if user-visible behavior changed.

## End of Session

1. Run `./init.sh`; record result in `feature_list.json` evidence.
2. Update `progress.md` (Last updated, active feature, next steps, risks) and `session-handoff.md`.
3. Commit only from a clean, passing state so the next session is restartable with `./init.sh` alone.

## Verification

```bash
./init.sh                 # full gate
go test ./internal/pgp ./internal/formcheck   # fast loop, no GUI libs needed
GOOS=windows CGO_ENABLED=0 go test -c ./internal/formcheck   # compile-check Windows rules (run under wine if available)
make windows              # cross-build dist/pgp-tools.exe (needs gcc-mingw-w64-x86-64)
make report               # reports/index.html; attach summary.md as evidence
```

## Escalate to user

- Changing key roles, default cipher, or output-overwrite semantics.
- Adding network access, telemetry, or key generation/management features.
