package miosa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Header names the API reads or writes on machine creates.
const (
	// HeaderBillTo selects the organization a create bills (id, slug or name).
	HeaderBillTo = "X-Miosa-Bill-To"
	// HeaderBillToSource says why a create billed the organization it did
	// (response header): "request", "setting" or "credential". The response
	// also carries X-Miosa-Bill-To, the organization id that was billed.
	HeaderBillToSource = "X-Miosa-Bill-To-Source"
	// HeaderWaitOutcome is "ready" or "timeout" after a create with ?wait=N.
	HeaderWaitOutcome = "X-Miosa-Wait"
)

// WithBillTo makes every request this client sends carry the per-request
// bill-to override (an organization id, slug or name). Only create calls act
// on it; see BillToService for the account-wide setting.
func WithBillTo(org string) ClientOption {
	return func(c *Client) {
		if org == "" {
			return
		}
		if c.defaultHeaders == nil {
			c.defaultHeaders = map[string]string{}
		}
		c.defaultHeaders[HeaderBillTo] = org
	}
}

// WithDefaultHeader makes every request carry the header.
func WithDefaultHeader(name, value string) ClientOption {
	return func(c *Client) {
		if name == "" {
			return
		}
		if c.defaultHeaders == nil {
			c.defaultHeaders = map[string]string{}
		}
		c.defaultHeaders[name] = value
	}
}

type requestHeadersKey struct{}

// WithRequestHeaders returns a context whose requests carry extra headers.
// They win over client defaults. Use it for a single call:
//
//	ctx = miosa.WithRequestHeaders(ctx, map[string]string{miosa.HeaderBillTo: "acme"})
func WithRequestHeaders(ctx context.Context, headers map[string]string) context.Context {
	merged := map[string]string{}
	for k, v := range requestHeadersFromContext(ctx) {
		merged[k] = v
	}
	for k, v := range headers {
		merged[k] = v
	}
	return context.WithValue(ctx, requestHeadersKey{}, merged)
}

// WithBillToContext returns a context whose create requests bill the named
// organization (an organization id, slug or name).
func WithBillToContext(ctx context.Context, org string) context.Context {
	return WithRequestHeaders(ctx, map[string]string{HeaderBillTo: org})
}

func requestHeadersFromContext(ctx context.Context) map[string]string {
	m, _ := ctx.Value(requestHeadersKey{}).(map[string]string)
	return m
}

// sendJSONResponse is sendJSONWithHeaders that also returns the response
// headers, for the calls whose headers carry data (x-miosa-wait,
// x-miosa-bill-to). The status code is returned too (202 vs 200 matter on a
// few routes).
func (c *Client) sendJSONResponse(ctx context.Context, method, path string, in, out interface{}, headers map[string]string) (http.Header, int, error) {
	var bodyReader io.ReadSeeker
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	resp, err := c.doWithHeaders(ctx, method, path, bodyReader, headers)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if out == nil || resp.ContentLength == 0 {
		return resp.Header, resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return resp.Header, resp.StatusCode, err
	}
	return resp.Header, resp.StatusCode, nil
}

// WithAccessToken authenticates with an OAuth access token or session JWT
// instead of an API key. It is sent as the bearer token.
func WithAccessToken(token string) ClientOption {
	return func(c *Client) { c.accessToken = token }
}

// WithTenant sends the organization to act in as X-MIOSA-Tenant. Use it with a
// credential that belongs to several organizations.
func WithTenant(tenantID string) ClientOption {
	return func(c *Client) { c.tenant = tenantID }
}

// WithUserAgent appends a product token to the user agent, for example
// "my-tool/1.2", so your traffic can be told apart in telemetry.
func WithUserAgent(suffix string) ClientOption {
	return func(c *Client) { c.userAgentSuffix = suffix }
}

// WithUserAgentOverride replaces the user agent entirely, for white-label
// products that must not announce the SDK. An empty value restores the default.
func WithUserAgentOverride(ua string) ClientOption {
	return func(c *Client) { c.userAgentOverride = ua }
}

// HeaderTenant is the header that selects the organization.
const HeaderTenant = "X-MIOSA-Tenant"

// bearer is the token sent in the Authorization header.
func (c *Client) bearer() string {
	if c.accessToken != "" {
		return c.accessToken
	}
	return c.apiKey
}

// userAgent is the User-Agent this client sends.
func (c *Client) userAgent() string {
	if c.userAgentOverride != "" {
		return c.userAgentOverride
	}
	ua := "miosa-go/" + sdkVersion
	if c.userAgentSuffix != "" {
		ua += " " + c.userAgentSuffix
	}
	return ua
}

// setAuthHeaders sets the headers every request carries: bearer token, user
// agent, tenant, client defaults, then per-call context headers (which win).
func (c *Client) setAuthHeaders(ctx context.Context, h http.Header) {
	h.Set("Authorization", "Bearer "+c.bearer())
	h.Set("User-Agent", c.userAgent())
	if c.tenant != "" {
		h.Set(HeaderTenant, c.tenant)
	}
	for k, v := range c.defaultHeaders {
		h.Set(k, v)
	}
	for k, v := range requestHeadersFromContext(ctx) {
		h.Set(k, v)
	}
}

// withoutTimeout returns a shallow copy of the client whose HTTP client has no
// overall timeout. Streaming calls use it so a long stream is not cut at the
// request timeout; the caller's context bounds them instead.
func (c *Client) withoutTimeout() *Client {
	if c.httpClient.Timeout == 0 {
		return c
	}
	hc := *c.httpClient
	hc.Timeout = 0
	cp := *c
	cp.httpClient = &hc
	return &cp
}
