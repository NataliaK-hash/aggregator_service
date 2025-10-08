package utils

// EmptyFallback returns the provided fallback string if value is empty.
func EmptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// Ptr returns a pointer to the provided value.
func Ptr[T any](v T) *T {
	return &v
}
