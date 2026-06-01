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
