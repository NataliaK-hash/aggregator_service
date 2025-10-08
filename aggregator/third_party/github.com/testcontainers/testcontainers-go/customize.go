package testcontainers

type Request struct {
	Image string
	Env   map[string]string
}

type CustomizeRequestOption func(*Request)

func WithImage(image string) CustomizeRequestOption {
	return func(r *Request) {
		r.Image = image
	}
}
