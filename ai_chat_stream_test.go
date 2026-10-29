package rootly

import (
	"context"
	"errors"
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
	const body = `{"errors":[{"title":"rate limited","status":"429"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(body))
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
	var statusErr *AiChatStreamStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error type = %T, want *AiChatStreamStatusError", err)
	}
	if statusErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusTooManyRequests)
	}
	if got := statusErr.Header.Get("Retry-After"); got != "17" {
		t.Errorf("Retry-After = %q, want %q", got, "17")
	}
	if got := string(statusErr.Body); got != body {
		t.Errorf("Body = %q, want %q", got, body)
	}
	if got, want := err.Error(), "unexpected status 429 Too Many Requests: "+body; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got, ok := statusErr.RetryAfter(); !ok || got != 17*time.Second {
		t.Errorf("RetryAfter() = %v, %v, want 17s, true", got, ok)
	}
}

func TestAiChatStreamStatusError_RetryAfter(t *testing.T) {
	futureDate := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	pastDate := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)

	tests := []struct {
		name      string
		value     string
		want      time.Duration
		wantOK    bool
		minimum   time.Duration
		maximum   time.Duration
		setHeader bool
	}{
		{name: "seconds", value: "17", want: 17 * time.Second, wantOK: true, setHeader: true},
		{name: "HTTP-date", value: futureDate, wantOK: true, minimum: 28 * time.Second, maximum: 30 * time.Second, setHeader: true},
		{name: "past date", value: pastDate, want: 0, wantOK: true, setHeader: true},
		{name: "missing", wantOK: false},
		{name: "garbage", value: "not a date", wantOK: false, setHeader: true},
		{name: "signed seconds", value: "+17", wantOK: false, setHeader: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statusErr := &AiChatStreamStatusError{Header: make(http.Header)}
			if test.setHeader {
				statusErr.Header.Set("Retry-After", test.value)
			}
			got, ok := statusErr.RetryAfter()
			if ok != test.wantOK {
				t.Fatalf("RetryAfter() ok = %v, want %v", ok, test.wantOK)
			}
			if test.name == "HTTP-date" {
				if got < test.minimum || got > test.maximum {
					t.Errorf("RetryAfter() = %v, want between %v and %v", got, test.minimum, test.maximum)
				}
				return
			}
			if got != test.want {
				t.Errorf("RetryAfter() = %v, want %v", got, test.want)
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
