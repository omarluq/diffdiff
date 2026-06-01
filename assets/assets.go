// Package assets embeds the application's bundled image assets so a built
// binary is self-contained.
package assets

import "embed"

// Icons holds the Material Icon Theme file- and folder-type icon PNGs under
// imgs/icons/. Only that subdirectory is embedded, so the README-only banner
// (imgs/readmebanner.png) stays out of the binary; the mascot is embedded
// separately as Mascot.
//
//go:embed imgs/icons/*.png
var Icons embed.FS

// Nyan is the animated nyan-cat GIF shown as the working-tree scan's loading
// indicator (see cmd/diffdiff's scan dialog).
//
//go:embed gifs/nyan-cat-poptart-cat.gif
var Nyan []byte

// Mascot is the application icon, set as the running window/taskbar icon via
// App.SetIcon and installed as the desktop launcher icon by `task install`.
//
//go:embed imgs/mascot.png
var Mascot []byte
