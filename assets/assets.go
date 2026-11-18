// Package assets embeds static resources so the binary stays self-contained.
package assets

import _ "embed"

//go:embed icon.svg
var IconSVG []byte

//go:embed pgp-tools.desktop
var DesktopEntry []byte
