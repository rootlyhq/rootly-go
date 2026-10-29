package rootly

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AI chat stream event types sent by StreamAiChat.
const (
	AiChatStreamEventSessionID            = "session_id"
	AiChatStreamEventText                 = "text"
	AiChatStreamEventStatus               = "status"
	AiChatStreamEventTaskUpdate           = "task_update"
	AiChatStreamEventSkillLoaded          = "skill_loaded"
	AiChatStreamEventConfirmationRequired = "confirmation_required"
	AiChatStreamEventCheckpointResolved   = "checkpoint_resolved"
	AiChatStreamEventError                = "error"
	AiChatStreamEventDone                 = "done"
)

// AiChatStreamEvent is a single Server-Sent Event from StreamAiChat.
type AiChatStreamEvent struct {
	// Type is the SSE event name, such as AiChatStreamEventText or AiChatStreamEventDone.
	Type string
	// Data is the event's JSON payload, such as `{"content":"..."}` for a `text` event.
	Data json.RawMessage
}

// Decode unmarshals the event's JSON payload into v, such as an *AiChatStreamText for a `text` event.
func (e AiChatStreamEvent) Decode(v any) error {
	return json.Unmarshal(e.Data, v)
}

// AiChatStreamSessionID is the payload of a `session_id` event, sent first, which can be passed as StreamAiChatParams.SessionID to continue the conversation.
type AiChatStreamSessionID struct {
	SessionID string `json:"session_id"`
}

// AiChatStreamText is the payload of a `text` event, a chunk of the AI's reply.
type AiChatStreamText struct {
	Content string `json:"content"`
}

// AiChatStreamStatus is the payload of a `status` event, a short progress line such as the running tool's title.
type AiChatStreamStatus struct {
	Status string `json:"status"`
}

// AiChatStreamTaskUpdate is the payload of a `task_update` event.
type AiChatStreamTaskUpdate struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Status  string          `json:"status"`
	Output  json.RawMessage `json:"output,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// AiChatStreamSkillLoaded is the payload of a `skill_loaded` event.
type AiChatStreamSkillLoaded struct {
	Skill struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		Version int    `json:"version"`
	} `json:"skill"`
}

// AiChatStreamConfirmationRequired is the payload of a `confirmation_required` event, sent when a write needs approval. Resolve it with a new StreamAiChat call setting StreamAiChatParams.CheckpointID and StreamAiChatParams.Decision.
type AiChatStreamConfirmationRequired struct {
	CheckpointID  string          `json:"checkpoint_id"`
	Question      string          `json:"question"`
	Preview       json.RawMessage `json:"preview,omitempty"`
	MissingFields json.RawMessage `json:"missing_fields,omitempty"`
	ExpiresAt     string          `json:"expires_at"`
}

// AiChatStreamCheckpointResolved is the payload of a `checkpoint_resolved` event.
type AiChatStreamCheckpointResolved struct {
	CheckpointID string  `json:"checkpoint_id"`
	Status       string  `json:"status"`
	Replay       *string `json:"replay"`
}

// AiChatStreamError is the payload of an `error` event.
type AiChatStreamError struct {
	Message string `json:"message"`
}

// AiChatStreamDone is the payload of the final `done` event, whose Status is `complete`, `confirmation_required`, `error`, or the status of a halted turn.
type AiChatStreamDone struct {
	Status string `json:"status"`
}

// AiChatStreamStatusError is returned by StreamAiChatEvents when the API responds with a non-2xx status, such as 401, 403 or 429.
type AiChatStreamStatusError struct {
	StatusCode int
	Header     http.Header
	// Body is the response body, usually a JSON:API `errors` document.
	Body []byte
}

func (e *AiChatStreamStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d %s: %s", e.StatusCode, http.StatusText(e.StatusCode), strings.TrimSpace(string(e.Body)))
}

// RetryAfter returns how long to wait before retrying, from the `Retry-After` header, or `false` if it is absent or invalid.
func (e *AiChatStreamStatusError) RetryAfter() (time.Duration, bool) {
	value := e.Header.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(time.Until(date), 0), true
	}
	return 0, false
}

// StreamAiChatEvents calls StreamAiChat and returns an iterator over the Server-Sent Events in the response, as they arrive.
//
// A non-2xx response is returned as an *AiChatStreamStatusError. The iterator closes the response body once iteration stops.
func (c *Client) StreamAiChatEvents(ctx context.Context, params *StreamAiChatParams, reqEditors ...RequestEditorFn) (iter.Seq2[AiChatStreamEvent, error], error) {
	reqEditors = append(reqEditors, func(_ context.Context, req *http.Request) error {
		req.Header.Set("Accept", "text/event-stream")
		return nil
	})

	rsp, err := c.StreamAiChat(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode < 200 || rsp.StatusCode > 299 {
		defer func() { _ = rsp.Body.Close() }()
		body, _ := io.ReadAll(rsp.Body)
		return nil, &AiChatStreamStatusError{StatusCode: rsp.StatusCode, Header: rsp.Header, Body: body}
	}

	return func(yield func(AiChatStreamEvent, error) bool) {
		defer func() { _ = rsp.Body.Close() }()
		for event, err := range readAiChatStreamEvents(rsp.Body) {
			if !yield(event, err) {
				return
			}
		}
	}, nil
}

// readAiChatStreamEvents parses Server-Sent Events from r, skipping comments such as `: ping` heartbeats.
func readAiChatStreamEvents(r io.Reader) iter.Seq2[AiChatStreamEvent, error] {
	return func(yield func(AiChatStreamEvent, error) bool) {
		scanner := bufio.NewScanner(r)
		// `text` and `task_update` events can be larger than bufio's 64KiB default
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

		var eventType string
		var data []string
		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				if len(data) > 0 {
					if eventType == "" {
						eventType = "message"
					}
					if !yield(AiChatStreamEvent{Type: eventType, Data: json.RawMessage(strings.Join(data, "\n"))}, nil) {
						return
					}
				}
				eventType, data = "", nil
				continue
			}

			if strings.HasPrefix(line, ":") {
				continue
			}

			field, value, _ := strings.Cut(line, ":")
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "event":
				eventType = value
			case "data":
				data = append(data, value)
			}
		}

		if err := scanner.Err(); err != nil {
			yield(AiChatStreamEvent{}, err)
		}
	}
}
