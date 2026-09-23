package rootly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithBearerToken("test-token"))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.ListIncidents(context.Background(), &ListIncidentsParams{})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
}
