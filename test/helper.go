package test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/rootlyhq/rootly-go"
)

func SetupClient(t *testing.T) *rootly.ClientWithResponses {
	t.Helper()

	apiToken := os.Getenv("ROOTLY_API_TOKEN")
	if apiToken == "" {
		t.Skip("Skipping integration test: ROOTLY_API_TOKEN not set")
	}

	client, err := rootly.NewClientWithResponses(rootly.ServerURLProduction, rootly.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+apiToken)
		return nil
	}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}
