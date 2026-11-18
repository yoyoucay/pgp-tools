# Progress

**Last Updated:** 2026-11-18
**Active feature:** feat-008 (Windows) - awaiting first real Windows CI run on the PR

## Done

- feat-001 crypto core, feat-002 UI, feat-003 non-root install, feat-007 unit test report. Evidence in `feature_list.json`.
- Fixed: missing key file now shows "Key file not found: <path>" (mapping was never applied; caught by new `internal/ui` tests).
- feat-006 drag and drop, feat-009 live validation. feat-008 Windows verified under Wine only.

## Next

1. On the PR, confirm `build` is green on all three runners: Windows job runs `TestRealShortcut` (PowerShell .lnk) and `scripts/build-windows.ps1`, and the exe has its version resource. Fix and mark feat-004/feat-008 done.
2. feat-005 release packaging.

## Decisions

- **Go + Fyne + ProtonMail/go-crypto** over Python/Tk or Electron: one self-contained binary, no GnuPG, no runtime to install, native Wayland/X11, Windows, macOS.
- **Key roles corrected** from the original brief (which had decrypt=public key, encrypt=private key+passphrase). OpenPGP encrypts to the recipient's public key and decrypts with the holder's private key. The private key + passphrase on Encrypt and public key on Decrypt are kept as optional sign/verify.
- **Output folder picker, not Fyne save dialog**: Fyne's save dialog truncates the chosen file immediately.
- **Self-install via flag** instead of a shell installer: the portable binary carries its own desktop entry and icon.
- **Windows shortcut via PowerShell + WScript.Shell** instead of a COM library or hand-written .lnk: no new dependency; failure leaves the exe installed and is reported.
- **windres (MinGW) for exe resources** instead of go-winres: already part of the required C toolchain, no extra tool download.
- **Validation in `internal/formcheck`**, not in widgets: rules are testable on every OS without a display.

## Risks

- Linux binary dynamically links libGL, libX11, libwayland-client (present on desktop distros). For X11-only targets build with `-tags x11`.
- Go strings holding passphrases cannot be zeroed; entry text is cleared after success.
- Symmetric (passphrase-only) messages are rejected with `ErrSymmetricOnly`; support is out of scope.
- Windows exe is not code-signed: SmartScreen warns on first run.
- Corporate PowerShell restrictions (Constrained Language Mode) block the Start Menu shortcut; app reports it and the exe stays installed.
- `internal/ui` unit coverage is low (widgets); validation logic lives in `formcheck` (89%).
