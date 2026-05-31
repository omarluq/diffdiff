package di

import "github.com/samber/do/v2"

// RegisterServices registers all application services in the injector.
func RegisterServices(injector do.Injector) {
	do.Provide(injector, NewConfigService)
	do.Provide(injector, NewLoggerService)
	do.Provide(injector, NewRepository)
	do.Provide(injector, NewHighlighter)
	do.Provide(injector, NewThemeRegistry)
}
