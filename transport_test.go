package miosa_test

import (
	"net/http"
	"reflect"
	"testing"
	"unsafe"

	"github.com/Miosa-osa/miosa-go"
)

// TestNewClientReusesHTTPClient verifies that two service references on the
// same Client share the underlying *http.Client (and therefore the same
// connection pool / TLS session cache).
func TestNewClientReusesHTTPClient(t *testing.T) {
	c := miosa.NewClient("msk_u_test")

	// All resource services hang off the same *Client, so they share the
	// same httpClient. Probe via reflection because httpClient is
	// unexported.
	v := reflect.ValueOf(c).Elem()
	f := v.FieldByName("httpClient")
	if !f.IsValid() {
		t.Fatal("Client.httpClient field not found via reflection")
	}
	// Make unexported field readable.
	ptr := unsafe.Pointer(f.UnsafeAddr())
	httpClient := *(**http.Client)(ptr)
	if httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	if httpClient.Transport == nil {
		t.Fatal("httpClient.Transport is nil — HTTP/2 setup did not run")
	}

	// Sanity: a second NewClient must build its own transport (so callers
	// who want isolation get it) — but two services on the *same* client
	// must reuse the same pool.
	c2 := miosa.NewClient("msk_u_other")
	v2 := reflect.ValueOf(c2).Elem()
	f2 := v2.FieldByName("httpClient")
	httpClient2 := *(**http.Client)(unsafe.Pointer(f2.UnsafeAddr()))
	if httpClient == httpClient2 {
		t.Fatal("NewClient must build its own *http.Client per construction")
	}
}

// TestDefaultTransportIsHTTP2Capable verifies the configured transport has
// the http2 settings baked in (TLSNextProto["h2"] is registered after
// http2.ConfigureTransport runs).
func TestDefaultTransportIsHTTP2Capable(t *testing.T) {
	c := miosa.NewClient("msk_u_test")

	v := reflect.ValueOf(c).Elem()
	f := v.FieldByName("httpClient")
	httpClient := *(**http.Client)(unsafe.Pointer(f.UnsafeAddr()))
	tr, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is not *http.Transport: %T", httpClient.Transport)
	}
	if !tr.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 is false — HTTP/2 negotiation may not happen")
	}
	if tr.MaxIdleConnsPerHost < 2 {
		t.Errorf("MaxIdleConnsPerHost = %d, want >= 2 for keep-alive reuse", tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout == 0 {
		t.Error("IdleConnTimeout is zero — idle connections will live forever (or be evicted by default)")
	}
}
