# AGENTS.md

PGP Tools: portable desktop app (Go + Fyne) to encrypt/decrypt files with OpenPGP keys. Pure-Go crypto (`github.com/ProtonMail/go-crypto`), no GnuPG dependency, single binary, per-user install without root.

## Startup Workflow

1. Read this file, then `README.md` and `docs/ARCHITECTURE.md`.
2. Run `./init.sh`. If it fails, fix the baseline before new scope.
3. Read `feature_list.json` and `progress.md`; `git log --oneline -5`.

## Layout

| Path | Owns |
|---|---|
| `internal/pgp` | Crypto core. No UI imports. All behavior covered by `crypto_test.go`. |
| `internal/install` | Per-user XDG install/uninstall. No root paths. |
| `internal/ui` | Fyne widgets only. Calls `pgp.Encrypt/Decrypt`; no crypto logic. |
| `cmd/pgp-tools` | Flags (`--install`, `--uninstall`, `--version`) and UI launch. |
| `assets` | Embedded icon + desktop entry template. |
| `tools/testreport` | `go test -json` to HTML/JUnit/Markdown report. Standard library only; not shipped in the app binary. |

## Invariants (do not break)

- Key roles: encrypt = recipient PUBLIC key (+ optional own private key/passphrase to sign); decrypt = own PRIVATE key + passphrase (+ optional sender public key to verify). Do not swap.
- Output is written to a temp file and renamed only after integrity/signature checks pass. No partial plaintext on failure or cancel.
- When a verify key is given, decrypt fails unless signed by that exact key.
- Passphrases are never persisted, logged, or shown. Only key file paths go to preferences.
- Nothing requires root. Install targets come from `XDG_*` or `~/.local`.
- UI updates from goroutines go through `fyne.Do`.
- New error cases: add a sentinel in `internal/pgp/errors.go`, map in `internal/ui/messages.go`, test in `crypto_test.go`.

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
go test ./internal/pgp    # fast loop for crypto changes (no GUI libs needed)
make report               # reports/index.html; attach summary.md as evidence
```

## Escalate to user

- Changing key roles, default cipher, or output-overwrite semantics.
- Adding network access, telemetry, or key generation/management features.
