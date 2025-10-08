package utils

func EmptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func Ptr[T any](v T) *T {
	return &v
}
