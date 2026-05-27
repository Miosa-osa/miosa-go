package miosa

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CompletionsService provides OpenAI-compatible text and chat completion
// endpoints under /api/v1/intelligence/.
type CompletionsService struct {
	client *Client
}

// CompletionRequest is the request body for POST /intelligence/completions.
type CompletionRequest struct {
	Model  string      `json:"model"`
	Prompt interface{} `json:"prompt,omitempty"` // string or []string
	Stream bool        `json:"stream,omitempty"`
	// Additional fields forwarded verbatim (temperature, max_tokens, etc.)
	Extra map[string]interface{} `json:"-"`
}

// ChatCompletionRequest is the request body for POST /intelligence/chat/completions.
type ChatCompletionRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Stream   bool                     `json:"stream,omitempty"`
	Extra    map[string]interface{}   `json:"-"`
}

// CompletionResponse is the JSON response from a non-streaming completion.
type CompletionResponse map[string]interface{}

// SSEEvent is a single server-sent event parsed from a streaming response.
type SSEEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	Raw  string          `json:"-"`
}

func mergeExtra(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// Create sends a text completion request. When req.Stream is true it returns
// a nil response and opens a goroutine-backed SSE channel on ch. When
// req.Stream is false ch is nil.
func (s *CompletionsService) Create(ctx context.Context, req CompletionRequest) (CompletionResponse, <-chan SSEEvent, error) {
	body := map[string]interface{}{
		"model":  req.Model,
		"stream": req.Stream,
	}
	if req.Prompt != nil {
		body["prompt"] = req.Prompt
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	if req.Stream {
		ch, err := s.client.streamSSE(ctx, http.MethodPost, "/intelligence/completions", body)
		return nil, ch, err
	}
	var out CompletionResponse
	if err := s.client.postJSON(ctx, "/intelligence/completions", body, &out); err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// Chat sends a chat completion request. Returns an SSE channel when Stream is
// set, otherwise a single response object.
func (s *CompletionsService) Chat(ctx context.Context, req ChatCompletionRequest) (CompletionResponse, <-chan SSEEvent, error) {
	body := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	if req.Stream {
		ch, err := s.client.streamSSE(ctx, http.MethodPost, "/intelligence/chat/completions", body)
		return nil, ch, err
	}
	var out CompletionResponse
	if err := s.client.postJSON(ctx, "/intelligence/chat/completions", body, &out); err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// streamSSE opens an SSE connection and returns a channel of events. The
// channel is closed when the stream ends or ctx is cancelled.
func (c *Client) streamSSE(ctx context.Context, method, path string, body interface{}) (<-chan SSEEvent, error) {
	resp, err := c.doSSERequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	ch := make(chan SSEEvent, 32)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		parseSSEStream(ctx, resp, ch)
	}()
	return ch, nil
}

// doSSERequest builds and executes a request returning an open SSE response.
func (c *Client) doSSERequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	resp, err := c.doWithHeaders(ctx, method, path, jsonReader(body), map[string]string{
		"Accept": "text/event-stream",
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// parseSSEStream reads an SSE response and forwards events to ch.
func parseSSEStream(ctx context.Context, resp *http.Response, ch chan<- SSEEvent) {
	scanner := bufio.NewScanner(resp.Body)
	var dataBuf strings.Builder
	var eventType string

	flush := func() {
		raw := strings.TrimSpace(dataBuf.String())
		dataBuf.Reset()
		if raw == "" || raw == "[DONE]" {
			return
		}
		ev := SSEEvent{Type: eventType, Raw: raw}
		if eventType == "" {
			ev.Type = "message"
		}
		eventType = ""
		ev.Data = json.RawMessage(raw)
		select {
		case ch <- ev:
		case <-ctx.Done():
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(payload)
		case strings.HasPrefix(line, ":"), strings.HasPrefix(line, "id:"),
			strings.HasPrefix(line, "retry:"):
			// Ignore.
		}
	}
	flush()
}

// jsonReader marshals v into a bytes.Reader for use as an HTTP body.
func jsonReader(v interface{}) *jsonBodyReader {
	b, err := json.Marshal(v)
	if err != nil {
		return &jsonBodyReader{err: err}
	}
	return &jsonBodyReader{data: b, pos: 0}
}

type jsonBodyReader struct {
	data []byte
	pos  int
	err  error
}

func (r *jsonBodyReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *jsonBodyReader) Seek(offset int64, whence int) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = int64(r.pos) + offset
	case 2:
		abs = int64(len(r.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}
	if abs < 0 {
		return 0, fmt.Errorf("negative seek")
	}
	r.pos = int(abs)
	return abs, nil
}
