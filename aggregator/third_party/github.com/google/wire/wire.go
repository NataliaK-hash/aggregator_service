package wire

// ProviderSet представляет собой набор провайдеров для генератора зависимостей.
type ProviderSet struct{}

// NewSet возвращает пустой набор провайдеров и предназначен исключительно для заглушечного использования.
func NewSet(_ ...interface{}) ProviderSet {
	return ProviderSet{}
}

// Build imitates the signature of the real Wire Build sentinel to keep the
// project compilable when the actual Wire code generator is not available.
// It never executes at runtime and merely satisfies compile-time checks.
func Build(_ ...interface{}) ProviderSet {
	return ProviderSet{}
}
