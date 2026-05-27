package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/Miosa-osa/miosa-go"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// newTestWSClient creates a *miosa.Client whose base URL points at srv.
func newTestWSClient(t *testing.T, srv *httptest.Server) *miosa.Client {
	t.Helper()
	t.Cleanup(srv.Close)
	// Convert http:// to the form the client expects; Subscribe will swap to ws://.
	return miosa.NewClient("msk_u_test",
		miosa.WithBaseURL(srv.URL),
		miosa.WithMaxRetries(0),
	)
}

func TestEventsSubscribe(t *testing.T) {
	frames := []miosa.Event{
		{
			Type:      "window.focus_changed",
			Timestamp: "2026-01-01T00:00:01Z",
			Payload:   miosa.WindowFocusChangedPayload{WindowID: "w1", PID: "42", Title: "Terminal"},
		},
		{
			Type:      "file.created",
			Timestamp: "2026-01-01T00:00:02Z",
			Payload:   miosa.FileCreatedPayload{Path: "/home/user/new.txt"},
		},
		{
			Type:      "process.started",
			Timestamp: "2026-01-01T00:00:03Z",
			Payload:   miosa.ProcessStartedPayload{PID: 1234, Cmd: "python", PPID: "1"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_ev/events", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Verify subscribe query param is present.
		if r.URL.Query().Get("subscribe") == "" {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "missing subscribe"))
			return
		}

		for _, frame := range frames {
			// Send as typed envelope so decodeEventPayload can reconstruct.
			payloadBytes, _ := json.Marshal(frame.Payload)
			msg, _ := json.Marshal(map[string]interface{}{
				"type":      string(frame.Type),
				"timestamp": frame.Timestamp,
				"payload":   json.RawMessage(payloadBytes),
			})
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
		// Close gracefully after sending all frames.
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
	})

	srv := httptest.NewServer(mux)
	client := newTestWSClient(t, srv)

	// Register the computer endpoint.
	mux.HandleFunc("/computers/cmp_ev", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{ID: "cmp_ev", Status: miosa.StatusRunning})
	})

	computer, err := client.Computers.Get(context.Background(), "cmp_ev")
	if err != nil {
		t.Fatalf("Get computer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := computer.Events.Subscribe(ctx, miosa.EventSubscribeOptions{
		Subscribe: []miosa.EventProducer{miosa.ProducerWindow, miosa.ProducerFile, miosa.ProducerProcess},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	var received []miosa.Event
	for ev := range stream.C {
		received = append(received, ev)
	}

	if len(received) != 3 {
		t.Errorf("want 3 events, got %d", len(received))
	}
	if received[0].Type != "window.focus_changed" {
		t.Errorf("event[0].Type: want window.focus_changed, got %s", received[0].Type)
	}
	if received[1].Type != "file.created" {
		t.Errorf("event[1].Type: want file.created, got %s", received[1].Type)
	}
	if received[2].Type != "process.started" {
		t.Errorf("event[2].Type: want process.started, got %s", received[2].Type)
	}

	// Verify typed payload decoding.
	if p, ok := received[0].Payload.(miosa.WindowFocusChangedPayload); ok {
		if p.WindowID != "w1" {
			t.Errorf("WindowFocusChanged.WindowID: want w1, got %s", p.WindowID)
		}
	} else {
		t.Errorf("event[0].Payload type: want WindowFocusChangedPayload, got %T", received[0].Payload)
	}

	if p, ok := received[1].Payload.(miosa.FileCreatedPayload); ok {
		if p.Path != "/home/user/new.txt" {
			t.Errorf("FileCreated.Path: want /home/user/new.txt, got %s", p.Path)
		}
	} else {
		t.Errorf("event[1].Payload type: want FileCreatedPayload, got %T", received[1].Payload)
	}
}

func TestEventsSubscribeAuthHeader(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_auth/events", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		conn.Close()
	})
	mux.HandleFunc("/computers/cmp_auth", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{ID: "cmp_auth", Status: miosa.StatusRunning})
	})

	srv := httptest.NewServer(mux)
	client := newTestWSClient(t, srv)

	computer, _ := client.Computers.Get(context.Background(), "cmp_auth")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := computer.Events.Subscribe(ctx, miosa.EventSubscribeOptions{
		Subscribe: []miosa.EventProducer{miosa.ProducerIdle},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	// Drain the channel.
	for range stream.C {
	}

	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("Authorization header: want Bearer ..., got %q", gotAuth)
	}
}

func TestEventsSubscribeQueryParams(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_qp/events", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
		conn.Close()
	})
	mux.HandleFunc("/computers/cmp_qp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{ID: "cmp_qp", Status: miosa.StatusRunning})
	})

	srv := httptest.NewServer(mux)
	client := newTestWSClient(t, srv)

	computer, _ := client.Computers.Get(context.Background(), "cmp_qp")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := computer.Events.Subscribe(ctx, miosa.EventSubscribeOptions{
		Subscribe:        []miosa.EventProducer{miosa.ProducerFile},
		Paths:            []string{"/home/user", "/workspace"},
		IdleThresholdSec: 60,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	for range stream.C {
	}

	if !strings.Contains(gotQuery, "subscribe=file") {
		t.Errorf("query: want subscribe=file, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "paths=") {
		t.Errorf("query: want paths=, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "idle_threshold_sec=60") {
		t.Errorf("query: want idle_threshold_sec=60, got %q", gotQuery)
	}
}
