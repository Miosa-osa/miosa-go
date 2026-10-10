package miosa_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestClientReadsCredentialsFromEnvironment(t *testing.T) {
	var gotAuth, gotTenant atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotTenant.Store(r.Header.Get(miosa.HeaderTenant))
		writeJSON(w, 200, map[string]interface{}{"auth": map[string]interface{}{"method": "api_key"}})
	}))
	defer srv.Close()
	t.Setenv("MIOSA_API_KEY", "msk_u_from_env")
	t.Setenv("MIOSA_BASE_URL", srv.URL)
	t.Setenv("MIOSA_TENANT", "tenant-from-env")

	client := miosa.NewClient("")
	if _, err := client.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth.Load() != "Bearer msk_u_from_env" || gotTenant.Load() != "tenant-from-env" {
		t.Errorf("auth=%v tenant=%v", gotAuth.Load(), gotTenant.Load())
	}
}

func TestExplicitArgumentsBeatEnvironment(t *testing.T) {
	envSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the environment base URL must not be used when WithBaseURL is given")
	}))
	defer envSrv.Close()
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		writeJSON(w, 200, map[string]interface{}{})
	}))
	defer srv.Close()
	t.Setenv("MIOSA_API_KEY", "msk_u_env")
	t.Setenv("MIOSA_BASE_URL", envSrv.URL)
	t.Setenv("MIOSA_ACCESS_TOKEN", "")

	client := miosa.NewClient("msk_u_explicit", miosa.WithBaseURL(srv.URL))
	if _, err := client.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth.Load() != "Bearer msk_u_explicit" {
		t.Errorf("Authorization = %v", gotAuth.Load())
	}
}

func TestAccessTokenTenantAndUserAgentOptions(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{},
		miosa.WithAccessToken("mat_oauth"), miosa.WithTenant("org-42"), miosa.WithUserAgent("my-tool/1.2"))

	if _, err := client.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := rec.Last(t).Header
	if h.Get("Authorization") != "Bearer mat_oauth" {
		t.Errorf("Authorization = %q; an access token replaces the API key", h.Get("Authorization"))
	}
	if h.Get(miosa.HeaderTenant) != "org-42" {
		t.Errorf("tenant = %q", h.Get(miosa.HeaderTenant))
	}
	if ua := h.Get("User-Agent"); !strings.HasPrefix(ua, "miosa-go/2.") || !strings.HasSuffix(ua, " my-tool/1.2") {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestEveryHTTPPathCarriesTheAuthHeaders(t *testing.T) {
	// Streaming and multipart calls build their own requests; they must send
	// the same identity as the JSON helpers.
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/exec/stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: exit\ndata: {\"exit_code\":0}\n\n"))
			return
		}
		writeJSON(w, 200, map[string]interface{}{})
	}, miosa.WithAccessToken("mat_x"), miosa.WithTenant("org-1"))

	ch, err := client.OpenComputers.Hosts.Events(context.Background())
	if err == nil {
		for range ch {
		}
	}
	got := rec.Last(t).Header
	if got.Get("Authorization") != "Bearer mat_x" || got.Get(miosa.HeaderTenant) != "org-1" {
		t.Errorf("SSE request headers = %v", got)
	}
}

func counting(status int, body interface{}, hdr map[string]string) (http.HandlerFunc, *int32) {
	var n int32
	return func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&n, 1)
		for k, v := range hdr {
			w.Header().Set(k, v)
		}
		writeJSON(w, status, body)
	}, &n
}

func TestRetryPolicyByMethod(t *testing.T) {
	serverErr := errorEnvelope("INTERNAL", "boom", nil)
	cases := []struct {
		name    string
		call    func(c *miosa.Client) error
		status  int
		body    interface{}
		want    int32 // requests the server must see
		explain string
	}{
		{"GET 500 is retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Get(bg(), "s"); return err }, 500, serverErr, 3, "reads are idempotent"},
		{"DELETE 503 is retried", func(c *miosa.Client) error { return c.Sandboxes.Delete(bg(), "s") }, 503, serverErr, 3, "deletes are idempotent"},
		{"POST 500 without a key is NOT retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Pause(bg(), "s"); return err }, 500, serverErr, 1, "a create could run twice"},
		{"POST 500 with an Idempotency-Key is retried", func(c *miosa.Client) error {
			_, err := c.Sandboxes.Create(bg(), miosa.CreateSandboxInput{IdempotencyKey: "k"})
			return err
		}, 500, serverErr, 3, "the key makes it safe"},
		{"POST 429 is retried: it was refused before running", func(c *miosa.Client) error { _, err := c.Sandboxes.Pause(bg(), "s"); return err }, 429, errorEnvelope("RATE_LIMITED", "slow", map[string]interface{}{"retry_after_ms": 1}), 3, "rate limited"},
		{"POST 409 marked retryable is retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Pause(bg(), "s"); return err }, 409,
			errorEnvelope("SANDBOX_STARTING", "starting", map[string]interface{}{"retryable": true, "retry_after_ms": 1}), 3, "the server said so"},
		{"POST 503 marked retryable is retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Pause(bg(), "s"); return err }, 503,
			errorEnvelope("UNAVAILABLE", "later", map[string]interface{}{"retryable": true, "retry_after_ms": 1}), 3, "the server said so"},
		{"GET 404 is never retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Get(bg(), "s"); return err }, 404, errorEnvelope("NOT_FOUND", "no", nil), 1, "client error"},
		{"POST 409 not retryable is not retried", func(c *miosa.Client) error { _, err := c.Sandboxes.Pause(bg(), "s"); return err }, 409, errorEnvelope("CONFLICT", "no", nil), 1, "conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, n := counting(tc.status, tc.body, nil)
			client, _ := newHandlerClient(t, handler, miosa.WithMaxRetries(2))
			if err := tc.call(client); err == nil {
				t.Fatal("expected an error")
			}
			if got := atomic.LoadInt32(n); got != tc.want {
				t.Errorf("server saw %d requests, want %d (%s)", got, tc.want, tc.explain)
			}
		})
	}
}

func TestPostConnectionErrorIsNotRetriedButGetIs(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		// Drop the connection with no response.
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer srv.Close()
	client := miosa.NewClient("k", miosa.WithBaseURL(srv.URL), miosa.WithMaxRetries(2), miosa.WithHTTPClient(&http.Client{Timeout: 2 * time.Second}))

	_, err := client.Sandboxes.Pause(bg(), "s")
	var conn *miosa.ConnectionError
	if !errors.As(err, &conn) {
		t.Fatalf("error = %#v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("POST was sent %d times after a dropped connection, want 1", got)
	}

	atomic.StoreInt32(&hits, 0)
	_, _ = client.Sandboxes.Get(bg(), "s")
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("GET was sent %d times, want 3", got)
	}
}
