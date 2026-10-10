package miosa_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func getSandbox(c *miosa.Client) error {
	_, err := c.Sandboxes.Get(context.Background(), "sbx_1")
	return err
}

func TestErrorEnvelopeFields(t *testing.T) {
	body := errorEnvelope("NOT_FOUND", "sandbox not found", map[string]interface{}{
		"details": map[string]interface{}{"id": "sbx_1"},
	})
	client, _ := newFixedClient(t, http.StatusNotFound, body)

	err := getSandbox(client)

	var nf *miosa.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error = %T, want *NotFoundError", err)
	}
	if nf.Message != "sandbox not found" {
		t.Errorf("Message = %q", nf.Message)
	}
	if nf.Code != "NOT_FOUND" {
		t.Errorf("Code = %q", nf.Code)
	}
	details, ok := nf.Details.(map[string]interface{})
	if !ok || details["id"] != "sbx_1" {
		t.Errorf("Details = %#v", nf.Details)
	}
	if got := miosa.ErrorCode(err); got != "NOT_FOUND" {
		t.Errorf("ErrorCode = %q", got)
	}
	if !miosa.IsCode(fmt.Errorf("wrapped: %w", err), "NOT_FOUND") {
		t.Error("IsCode must see through %w wrapping")
	}
}

func TestErrorLegacyStringEnvelopeKeepsMessageAndCode(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusConflict, map[string]interface{}{"error": "name taken", "code": "NAME_TAKEN"})

	err := getSandbox(client)

	m := miosa.AsMiosaError(err)
	if m == nil {
		t.Fatalf("AsMiosaError(%T) = nil", err)
	}
	if m.Message != "name taken" || m.Code != "NAME_TAKEN" || m.StatusCode != 409 {
		t.Errorf("got %+v", m)
	}
}

func TestErrorObjectWithoutMessageDoesNotLeakRawJSON(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusBadRequest, map[string]interface{}{"error": map[string]interface{}{"code": "X"}})

	m := miosa.AsMiosaError(getSandbox(client))

	if m == nil || m.Message != "request failed with status 400" {
		t.Fatalf("message = %+v", m)
	}
}

func TestStartingErrorIsTypedAndRetryable(t *testing.T) {
	body := errorEnvelope("SANDBOX_STARTING", "the sandbox is still starting; retry shortly",
		map[string]interface{}{"retryable": true, "retry_after_ms": 2000})
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusConflict, body)
	})

	err := getSandbox(client)

	var starting *miosa.ResourceStartingError
	if !errors.As(err, &starting) {
		t.Fatalf("error = %T, want *ResourceStartingError", err)
	}
	if !miosa.IsStarting(err) {
		t.Error("IsStarting = false")
	}
	if !starting.Retryable || !starting.ShouldRetry() {
		t.Error("starting error must be retryable")
	}
	if d, ok := miosa.RetryAfter(err); !ok || d != 2*time.Second {
		t.Errorf("RetryAfter = %v, %v", d, ok)
	}
	if starting.Code != miosa.CodeSandboxStarting || starting.StatusCode != 409 {
		t.Errorf("got %+v", starting.MiosaError)
	}
}

func TestComputerStartingIsAlsoStarting(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusServiceUnavailable, errorEnvelope("COMPUTER_STARTING", "starting", nil))

	if !miosa.IsStarting(getSandbox(client)) {
		t.Error("COMPUTER_STARTING must map to ResourceStartingError")
	}
}

func TestRetryDelayFromBodyMillisecondsAndHeaderSeconds(t *testing.T) {
	cases := []struct {
		name   string
		header string
		extra  map[string]interface{}
		want   time.Duration
	}{
		{"body ms", "", map[string]interface{}{"retry_after_ms": 1500}, 1500 * time.Millisecond},
		{"header seconds", "3", nil, 3 * time.Second},
		{"header wins over body", "1", map[string]interface{}{"retry_after_ms": 9000}, time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
				if tc.header != "" {
					w.Header().Set("Retry-After", tc.header)
				}
				writeJSON(w, http.StatusServiceUnavailable, errorEnvelope("UNAVAILABLE", "later", tc.extra))
			})
			d, ok := miosa.RetryAfter(getSandbox(client))
			if !ok || d != tc.want {
				t.Fatalf("RetryAfter = %v, %v; want %v", d, ok, tc.want)
			}
		})
	}
}

func TestRateLimitErrorReadsRetryAfterHeader(t *testing.T) {
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		writeJSON(w, http.StatusTooManyRequests, errorEnvelope("RATE_LIMITED", "slow down", nil))
	})

	var rl *miosa.RateLimitError
	if err := getSandbox(client); !errors.As(err, &rl) {
		t.Fatalf("error = %T", err)
	}
	if rl.RetryAfter != 7 {
		t.Errorf("RetryAfter = %v, want 7", rl.RetryAfter)
	}
}

func TestSnapshotInUseErrorListsDependents(t *testing.T) {
	body := errorEnvelope("SNAPSHOT_IN_USE", "in use", map[string]interface{}{
		"details": map[string]interface{}{"dependents": []map[string]interface{}{
			{"type": "named_snapshot", "id": "n1", "label": "web-stack"},
			{"type": "child_snapshot", "id": "c1"},
		}},
	})
	client, _ := newFixedClient(t, http.StatusConflict, body)

	err := client.Snapshots.Delete(context.Background(), "snap_1")

	var inUse *miosa.SnapshotInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("error = %T", err)
	}
	if len(inUse.Dependents) != 2 || inUse.Dependents[0].Label != "web-stack" || inUse.Dependents[1].Type != "child_snapshot" {
		t.Errorf("dependents = %+v", inUse.Dependents)
	}
}

func TestEnvironmentConflictErrors(t *testing.T) {
	t.Run("memory withheld is not retryable and lists reasons", func(t *testing.T) {
		body := errorEnvelope("ENVIRONMENT_MEMORY_WITHHELD", "withheld", map[string]interface{}{
			"retryable": false,
			"details":   map[string]interface{}{"reasons": []string{"protected_copy", "other_workspace"}},
		})
		client, _ := newFixedClient(t, http.StatusConflict, body)

		_, err := client.Sandboxes.ForkSandbox(context.Background(), "sbx_1", miosa.PublicForkSandboxInput{})

		var w *miosa.EnvironmentMemoryWithheldError
		if !errors.As(err, &w) {
			t.Fatalf("error = %T", err)
		}
		if w.Retryable || len(w.Reasons) != 2 || w.Reasons[0] != "protected_copy" {
			t.Errorf("got %+v", w)
		}
	})
	t.Run("scrub failed is retryable", func(t *testing.T) {
		client, _ := newFixedClient(t, http.StatusConflict, errorEnvelope("ENVIRONMENT_SCRUB_FAILED", "could not empty the guest", nil))

		_, err := client.Sandboxes.ForkSandbox(context.Background(), "sbx_1", miosa.PublicForkSandboxInput{})

		var s *miosa.EnvironmentScrubFailedError
		if !errors.As(err, &s) || !s.Retryable {
			t.Fatalf("error = %T %+v", err, s)
		}
	})
}

func TestClientHonorsServerRetryHint(t *testing.T) {
	calls := 0
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, http.StatusTooManyRequests, errorEnvelope("RATE_LIMITED", "slow", map[string]interface{}{"retry_after_ms": 40}))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"id": "sbx_1"}})
	}, miosa.WithMaxRetries(2))

	start := time.Now()
	if err := getSandbox(client); err != nil {
		t.Fatalf("expected success after retry: %v", err)
	}
	if rec.Count() != 2 {
		t.Errorf("requests = %d, want 2", rec.Count())
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("retried after %v, before the 40ms hint", elapsed)
	}
}

func TestClientDoesNotWaitOutAHintLongerThanTheCeiling(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		writeJSON(w, http.StatusTooManyRequests, errorEnvelope("RATE_LIMITED", "slow", nil))
	}, miosa.WithMaxRetries(3))

	start := time.Now()
	err := getSandbox(client)

	var rl *miosa.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %T", err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d, want 1 (a 120s hint is returned to the caller)", rec.Count())
	}
	if time.Since(start) > 5*time.Second {
		t.Error("client slept through a hint above its ceiling")
	}
}
