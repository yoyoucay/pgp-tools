# Progress

**Last Updated:** 2026-11-18
**Active feature:** feat-004 (CI) - awaiting first GitHub run

## Done

- feat-001 crypto core, feat-002 UI, feat-003 non-root install. Evidence in `feature_list.json`.

## Next

1. Push, confirm `build` workflow green on all three OS runners; fix Windows/macOS cgo issues if any.
2. feat-005 release packaging.

## Decisions

- **Go + Fyne + ProtonMail/go-crypto** over Python/Tk or Electron: one self-contained binary, no GnuPG, no runtime to install, native Wayland/X11, Windows, macOS.
- **Key roles corrected** from the original brief (which had decrypt=public key, encrypt=private key+passphrase). OpenPGP encrypts to the recipient's public key and decrypts with the holder's private key. The private key + passphrase on Encrypt and public key on Decrypt are kept as optional sign/verify.
- **Output folder picker, not Fyne save dialog**: Fyne's save dialog truncates the chosen file immediately.
- **Self-install via flag** instead of a shell installer: the portable binary carries its own desktop entry and icon.

## Risks

- Linux binary dynamically links libGL, libX11, libwayland-client (present on desktop distros). For X11-only targets build with `-tags x11`.
- Go strings holding passphrases cannot be zeroed; entry text is cleared after success.
- Symmetric (passphrase-only) messages are rejected with `ErrSymmetricOnly`; support is out of scope.
