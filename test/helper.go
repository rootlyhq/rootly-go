package test

import (
	"os"
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"
	"github.com/rootlyhq/rootly-go"
)

func SetupClient(t *testing.T) *rootly.ClientWithResponses {
	t.Helper()

	apiToken := os.Getenv("ROOTLY_API_TOKEN")
	if apiToken == "" {
		t.Skip("Skipping integration test: ROOTLY_API_TOKEN not set")
	}

	authFn, err := securityprovider.NewSecurityProviderBearerToken(apiToken)
	if err != nil {
		t.Fatalf("Failed to create SecurityProvider: %v", err)
	}

	client, err := rootly.NewClientWithResponses(rootly.ServerURLProduction, rootly.WithRequestEditorFn(authFn.Intercept))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}
