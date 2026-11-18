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

## Development

See `AGENTS.md` (contributor/agent workflow), `docs/ARCHITECTURE.md`, and run `./init.sh` before and after changes.
