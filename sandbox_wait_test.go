package miosa_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Miosa-osa/miosa-go"
)

// streamingSSE writes an SSE response with the given event lines and flushes
// each chunk so the client can process them in real time.
func streamingSSE(w http.ResponseWriter, lines ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	for _, line := range lines {
		_, _ = w.Write([]byte(line))
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func TestSandboxWaitUntilReady_StreamReady(t *testing.T) {
	const id = "sbx_123"

	var streamHits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/"+id+"/readiness/stream", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&streamHits, 1)
		streamingSSE(w, ": keepalive\n\n", "event: ready\ndata: {\"ready_at\":\"2026-05-18T00:00:00Z\"}\n\n")
	})

	client := newTestClient(t, mux)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ready, err := client.Sandboxes.WaitUntilReady(ctx, id, miosa.WaitUntilReadyOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("WaitUntilReady returned error: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready=true, got false")
	}
	if got := atomic.LoadInt32(&streamHits); got != 1 {
		t.Fatalf("expected 1 stream hit, got %d", got)
	}
}

func TestSandboxWaitUntilReady_StreamTimeoutEvent(t *testing.T) {
	const id = "sbx_456"

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/"+id+"/readiness/stream", func(w http.ResponseWriter, r *http.Request) {
		streamingSSE(w, "event: timeout\ndata: {\"reason\":\"not_ready_after_30s\"}\n\n")
	})

	client := newTestClient(t, mux)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ready, err := client.Sandboxes.WaitUntilReady(ctx, id, miosa.WaitUntilReadyOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("WaitUntilReady returned error: %v", err)
	}
	if ready {
		t.Fatalf("expected ready=false on event: timeout, got true")
	}
}

func TestSandboxWaitUntilReady_404FallsBackToPolling(t *testing.T) {
	const id = "sbx_789"

	var streamHits int32
	var pollHits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/"+id+"/readiness/stream", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&streamHits, 1)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not implemented"})
	})
	mux.HandleFunc("/sandboxes/"+id+"/readiness", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&pollHits, 1)
		// First call: not ready. Second call: ready.
		ready := n >= 2
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{"ready": ready},
		})
	})

	client := newTestClient(t, mux)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ready, err := client.Sandboxes.WaitUntilReady(ctx, id, miosa.WaitUntilReadyOptions{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("WaitUntilReady returned error: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready=true via polling fallback, got false")
	}
	if got := atomic.LoadInt32(&streamHits); got != 1 {
		t.Fatalf("expected 1 stream attempt before fallback, got %d", got)
	}
	if got := atomic.LoadInt32(&pollHits); got < 2 {
		t.Fatalf("expected >=2 polling hits, got %d", got)
	}
}

func TestSandboxWaitUntilReady_StreamFalseSkipsSSE(t *testing.T) {
	const id = "sbx_no_sse"

	var streamHits int32
	var pollHits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/"+id+"/readiness/stream", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&streamHits, 1)
		streamingSSE(w, "event: ready\ndata: {}\n\n")
	})
	mux.HandleFunc("/sandboxes/"+id+"/readiness", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pollHits, 1)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{"ready": true},
		})
	})

	client := newTestClient(t, mux)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	streamOff := false
	ready, err := client.Sandboxes.WaitUntilReady(ctx, id, miosa.WaitUntilReadyOptions{
		Timeout: 2 * time.Second,
		Stream:  &streamOff,
	})
	if err != nil {
		t.Fatalf("WaitUntilReady returned error: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready=true via polling, got false")
	}
	if got := atomic.LoadInt32(&streamHits); got != 0 {
		t.Fatalf("expected 0 stream hits when stream=false, got %d", got)
	}
	if got := atomic.LoadInt32(&pollHits); got < 1 {
		t.Fatalf("expected at least 1 polling hit, got %d", got)
	}
}
