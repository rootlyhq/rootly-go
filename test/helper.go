package test

import (
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

	client, err := rootly.NewClientWithResponses(rootly.ServerURLProduction, rootly.WithBearerToken(apiToken))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}
