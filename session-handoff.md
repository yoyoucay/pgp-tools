# Session Handoff

## Current Objective

- Goal: portable OpenPGP encrypt/decrypt desktop app, per-user install without root.
- Status: feat-001..003 done; feat-004 (CI) written, unverified on GitHub.
- Branch: `claude/eager-lamport-051nqz`

## Verification Evidence

| Check | Command | Result |
|---|---|---|
| Format, vet, race tests, build | `./init.sh` | pass |
| GnuPG interop | `go test -run Interop ./internal/pgp` | pass (gpg 2.x) |
| GUI decrypt of GnuPG-signed file | Xvfb + xdotool | plaintext written 0600, signature valid |
| Per-user install | `HOME=<tmp> bin/pgp-tools --install` then `--uninstall` | files created 755/644, removed |

## Next Session Startup

1. Read `AGENTS.md`, `feature_list.json`, `progress.md`.
2. Run `./init.sh`.
3. Check the latest `build` workflow run; continue feat-004.
