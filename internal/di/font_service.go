package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/diffdiff/internal/theme"
)

// NewFontRegistry builds the registry of bundled monospace programming fonts.
func NewFontRegistry(_ do.Injector) (*theme.FontRegistry, error) {
	return theme.NewFontRegistry(), nil
}
