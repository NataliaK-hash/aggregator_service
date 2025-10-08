package testcontainers

// Request описывает упрощённую конфигурацию контейнера в заглушке TestContainers.
type Request struct {
	Image string
	Env   map[string]string
}

// CustomizeRequestOption изменяет конфигурацию запроса.
type CustomizeRequestOption func(*Request)

// WithImage задаёт имя образа контейнера.
func WithImage(image string) CustomizeRequestOption {
	return func(r *Request) {
		r.Image = image
	}
}
