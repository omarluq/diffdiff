package di

import (
	"github.com/samber/do/v2"

	"github.com/omarluq/diffdiff/internal/theme"
)

// NewThemeRegistry builds the curated registry of editor themes.
func NewThemeRegistry(_ do.Injector) (*theme.Registry, error) {
	return theme.NewRegistry(), nil
}
