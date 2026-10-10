package miosa_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// recorded is one request the test server saw.
type recorded struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Raw      []byte
}

// Body decodes the JSON body into a map (nil when there is none).
func (r recorded) Body(t *testing.T) map[string]interface{} {
	t.Helper()
	if len(r.Raw) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(r.Raw, &m); err != nil {
		t.Fatalf("request body is not a JSON object: %v (%s)", err, r.Raw)
	}
	return m
}

// recorder collects requests.
type recorder struct {
	mu   sync.Mutex
	reqs []recorded
}

func (r *recorder) add(req *http.Request) {
	raw, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, recorded{
		Method: req.Method, Path: req.URL.Path, RawQuery: req.URL.RawQuery,
		Header: req.Header.Clone(), Raw: raw,
	})
}

// Last is the most recent request.
func (r *recorder) Last(t *testing.T) recorded {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	return r.reqs[len(r.reqs)-1]
}

// Count is how many requests the server saw.
func (r *recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.reqs)
}

// newFixedClient answers every request with status and the JSON value body.
func newFixedClient(t *testing.T, status int, body interface{}, opts ...miosa.ClientOption) (*miosa.Client, *recorder) {
	t.Helper()
	return newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if body == nil {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, body)
	}, opts...)
}

// newHandlerClient runs handler for every request and records them.
func newHandlerClient(t *testing.T, handler http.HandlerFunc, opts ...miosa.ClientOption) (*miosa.Client, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	all := append([]miosa.ClientOption{
		miosa.WithBaseURL(srv.URL),
		miosa.WithMaxRetries(0),
	}, opts...)
	return miosa.NewClient("msk_u_test", all...), rec
}

// assertReq fails unless the last request has the method and path.
func assertReq(t *testing.T, got recorded, method, path string) {
	t.Helper()
	if got.Method != method || got.Path != path {
		t.Fatalf("request = %s %s, want %s %s", got.Method, got.Path, method, path)
	}
}

// errorEnvelope is the canonical API error body.
func errorEnvelope(code, message string, extra map[string]interface{}) map[string]interface{} {
	e := map[string]interface{}{"code": code, "message": message}
	for k, v := range extra {
		e[k] = v
	}
	return map[string]interface{}{"ok": false, "error": e}
}
