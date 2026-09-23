package rootly

import (
	"context"
	"net/http"
)

// WithBearerToken adds a bearer token to requests made by the client.
func WithBearerToken(token string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}
