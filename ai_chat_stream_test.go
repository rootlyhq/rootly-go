package rootly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamAiChatEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("Method = %q, want %q", req.Method, http.MethodPost)
		}
		if got := req.URL.Query().Get("message"); got != "hello" {
			t.Errorf("message = %q, want %q", got, "hello")
		}
		if got := req.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q, want %q", got, "text/event-stream")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: session_id\ndata: {\"session_id\":\"abc\"}\n\n" +
			": ping\n\n" +
			"event: text\ndata: {\"content\":\"Hi\"}\n\n" +
			"event: done\ndata: {\"status\":\"complete\"}\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	message := "hello"
	events, err := client.StreamAiChatEvents(context.Background(), &StreamAiChatParams{Message: &message})
	if err != nil {
		t.Fatal(err)
	}

	var got []AiChatStreamEvent
	for event, err := range events {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, event)
	}

	want := []AiChatStreamEvent{
		{Type: AiChatStreamEventSessionID, Data: []byte(`{"session_id":"abc"}`)},
		{Type: AiChatStreamEventText, Data: []byte(`{"content":"Hi"}`)},
		{Type: AiChatStreamEventDone, Data: []byte(`{"status":"complete"}`)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Type != want[i].Type || string(got[i].Data) != string(want[i].Data) {
			t.Errorf("event %d = {%s %s}, want {%s %s}", i, got[i].Type, got[i].Data, want[i].Type, want[i].Data)
		}
	}
}

func TestStreamAiChatEvents_StopsEarly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("event: text\ndata: {\"content\":\"a\"}\n\nevent: text\ndata: {\"content\":\"b\"}\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	events, err := client.StreamAiChatEvents(context.Background(), &StreamAiChatParams{})
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, err := range events {
		if err != nil {
			t.Fatal(err)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("got %d events, want 1", count)
	}
}

func TestStreamAiChatEvents_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"title":"message is required","status":"422"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.StreamAiChatEvents(context.Background(), &StreamAiChatParams{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "message is required") {
		t.Errorf("error = %q, want it to contain the status and body", err)
	}

	var statusErr *AiChatStreamStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T, want *AiChatStreamStatusError", err)
	}
	if statusErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(string(statusErr.Body), "message is required") {
		t.Errorf("Body = %q, want it to contain the error", statusErr.Body)
	}
	if _, ok := statusErr.RetryAfter(); ok {
		t.Error("RetryAfter() ok = true, want false without a Retry-After header")
	}
}

func TestStreamAiChatEvents_RetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":[{"title":"AI chat rate limit exceeded. Maximum 60 requests per minute.","status":"429"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.StreamAiChatEvents(context.Background(), &StreamAiChatParams{})
	if err == nil {
		t.Fatal("expected an error")
	}
	err = fmt.Errorf("stream: %w", err)

	var statusErr *AiChatStreamStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("errors.As(%v) = false, want *AiChatStreamStatusError", err)
	}
	if statusErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusTooManyRequests)
	}
	if got, ok := statusErr.RetryAfter(); !ok || got != 30*time.Second {
		t.Errorf("RetryAfter() = %v, %v, want 30s, true", got, ok)
	}
	if got := statusErr.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("X-RateLimit-Remaining = %q, want %q", got, "0")
	}
	if !strings.Contains(statusErr.Error(), "429 Too Many Requests") || !strings.Contains(statusErr.Error(), "AI chat rate limit exceeded") {
		t.Errorf("Error() = %q, want status and rate-limit message", statusErr.Error())
	}
}

func TestStreamAiChatEvents_RetryAfterDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.StreamAiChatEvents(context.Background(), &StreamAiChatParams{})
	var statusErr *AiChatStreamStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want *AiChatStreamStatusError", err)
	}
	if statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusServiceUnavailable)
	}
	if got, ok := statusErr.RetryAfter(); !ok || got <= 0 || got > 90*time.Second {
		t.Errorf("RetryAfter() = %v, %v, want > 0 and <= 90s, true", got, ok)
	}
}

func TestAiChatStreamStatusError_RetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
		wantOK bool
	}{
		{name: "absent"},
		{name: "seconds", header: "120", want: 2 * time.Minute, wantOK: true},
		{name: "zero", header: "0", want: 0, wantOK: true},
		{name: "whitespace", header: " 45 ", want: 45 * time.Second, wantOK: true},
		{name: "date in the past", header: "Wed, 21 Oct 2015 07:28:00 GMT", want: 0, wantOK: true},
		{name: "negative", header: "-1"},
		{name: "overflow", header: "9300000000"},
		{name: "invalid", header: "soon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AiChatStreamStatusError{Header: http.Header{}}
			if tt.header != "" {
				e.Header.Set("Retry-After", tt.header)
			}

			got, ok := e.RetryAfter()
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("RetryAfter() = %v, %v, want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}

	t.Run("date in the future", func(t *testing.T) {
		e := &AiChatStreamStatusError{Header: http.Header{}}
		e.Header.Set("Retry-After", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))

		got, ok := e.RetryAfter()
		if !ok || got <= 59*time.Minute || got > time.Hour {
			t.Errorf("RetryAfter() = %v, %v, want about 1h, true", got, ok)
		}
	})
}

func TestAiChatStreamStatusError_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{
			name:       "status and body",
			statusCode: 422,
			body:       `{"errors":[]}`,
			want:       `unexpected status 422 Unprocessable Entity: {"errors":[]}`,
		},
		{
			name:       "unknown status and trimmed body",
			statusCode: 599,
			body:       " oops\n",
			want:       "unexpected status 599: oops",
		},
		{
			name:       "empty body",
			statusCode: 502,
			want:       "unexpected status 502 Bad Gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AiChatStreamStatusError{StatusCode: tt.statusCode, Body: []byte(tt.body)}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadAiChatStreamEvents_MultiLineData(t *testing.T) {
	r := strings.NewReader("data: {\"a\":\ndata: 1}\n\n")

	var got []AiChatStreamEvent
	for event, err := range readAiChatStreamEvents(r) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, event)
	}

	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Type != "message" || string(got[0].Data) != "{\"a\":\n1}" {
		t.Errorf("event = {%s %s}, want {message {\"a\":\\n1}}", got[0].Type, got[0].Data)
	}
}

func TestAiChatStreamEvent_Decode(t *testing.T) {
	event := AiChatStreamEvent{
		Type: AiChatStreamEventConfirmationRequired,
		Data: []byte(`{"checkpoint_id":"abc","question":"Create the action item?","preview":{"title":"Fix it"},"missing_fields":null,"expires_at":"2026-10-29T12:00:00.000Z"}`),
	}

	var payload AiChatStreamConfirmationRequired
	if err := event.Decode(&payload); err != nil {
		t.Fatal(err)
	}

	if payload.CheckpointID != "abc" || payload.Question != "Create the action item?" || string(payload.Preview) != `{"title":"Fix it"}` {
		t.Errorf("payload = %+v", payload)
	}
}
