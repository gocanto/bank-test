package fees

type Response[T any] struct {
	Data T `json:"data"`
}

func respond[T any](value T) *Response[T] {
	return &Response[T]{Data: value}
}
