package rootly

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
