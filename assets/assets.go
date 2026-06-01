// Package assets embeds the application's bundled image assets so a built
// binary is self-contained.
package assets

import "embed"

// Icons holds the Material Icon Theme file- and folder-type icon PNGs under
// imgs/icons/. Only that subdirectory is embedded, so the larger README-only
// images alongside it (imgs/mascot.png, imgs/readmebanner.png) stay out of the
// binary.
//
//go:embed imgs/icons/*.png
var Icons embed.FS

// Nyan is the animated nyan-cat GIF shown as the working-tree scan's loading
// indicator (see cmd/diffdiff's scan dialog).
//
//go:embed gifs/nyan-cat-poptart-cat.gif
var Nyan []byte
