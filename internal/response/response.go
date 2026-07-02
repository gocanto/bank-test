// Package response provides a generic envelope for API responses. It is
// framework-agnostic and reusable by any service that wants to wrap its
// payloads under a top-level "data" key.
package response

type Response[T any] struct {
	Data T `json:"data"`
}

func Respond[T any](value T) *Response[T] {
	return &Response[T]{Data: value}
}
