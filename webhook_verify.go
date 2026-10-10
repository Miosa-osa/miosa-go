package miosa

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Receiving webhooks. MIOSA signs every delivery:
//
//	X-Miosa-Signature: sha256=<hex HMAC-SHA256 of the raw request body, keyed by the webhook secret>
//
// alongside X-Miosa-Event and X-Miosa-Delivery. Verify the signature against
// the exact bytes received, before parsing. VerifyWebhook, ParseWebhook and
// WebhookHandler do that.

// Headers a webhook delivery carries.
const (
	WebhookSignatureHeader = "X-Miosa-Signature"
	WebhookEventHeader     = "X-Miosa-Event"
	WebhookDeliveryHeader  = "X-Miosa-Delivery"
)

// Webhook verification errors.
var (
	// ErrWebhookSignatureMissing: the signature header is absent or empty.
	ErrWebhookSignatureMissing = errors.New("miosa: webhook signature header is missing")
	// ErrWebhookSignatureInvalid: the header is not "sha256=<hex>".
	ErrWebhookSignatureInvalid = errors.New("miosa: webhook signature is malformed")
	// ErrWebhookSignatureMismatch: the signature does not match the body.
	ErrWebhookSignatureMismatch = errors.New("miosa: webhook signature does not match")
	// ErrWebhookTooOld: the event is older than the tolerance allows.
	ErrWebhookTooOld = errors.New("miosa: webhook event is older than the tolerance")
)

// SignWebhook returns the X-Miosa-Signature value for a body: "sha256=" and the
// hex HMAC-SHA256. Useful for tests and for forwarding events.
func SignWebhook(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook checks a delivery's signature header against its raw body in
// constant time.
func VerifyWebhook(body []byte, signatureHeader, secret string) error {
	signatureHeader = strings.TrimSpace(signatureHeader)
	if signatureHeader == "" {
		return ErrWebhookSignatureMissing
	}
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return ErrWebhookSignatureInvalid
	}
	got, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, "sha256="))
	if err != nil || len(got) != sha256.Size {
		return ErrWebhookSignatureInvalid
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return ErrWebhookSignatureMismatch
	}
	return nil
}

// WebhookEvent is a verified, parsed delivery.
type WebhookEvent struct {
	// ID and DeliveryID are the delivery id (the same value). Use it to
	// de-duplicate: a delivery can arrive more than once.
	ID         string `json:"id"`
	DeliveryID string `json:"delivery_id"`
	// Event and Type are the event name, for example "sandbox.created".
	Event      string `json:"event"`
	Type       string `json:"type"`
	OccurredAt string `json:"occurred_at"`
	Timestamp  string `json:"timestamp"`
	TenantID   string `json:"tenant_id"`
	// Data is the event payload.
	Data json.RawMessage `json:"data"`
	// Raw is the whole verified body.
	Raw []byte `json:"-"`
}

// Decode unmarshals the event payload into v.
func (e *WebhookEvent) Decode(v interface{}) error { return json.Unmarshal(e.Data, v) }

// OccurredTime parses OccurredAt.
func (e *WebhookEvent) OccurredTime() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, e.OccurredAt)
}

// ParseWebhookOptions tune ParseWebhook.
type ParseWebhookOptions struct {
	// Tolerance, when positive, rejects events whose occurred_at is older than
	// this (ErrWebhookTooOld). Leave zero to accept replays and late retries.
	Tolerance time.Duration
	// Now overrides the clock, for tests.
	Now func() time.Time
}

// ParseWebhook verifies a delivery and parses it. body must be the raw bytes
// exactly as received.
func ParseWebhook(body []byte, signatureHeader, secret string, opts ...ParseWebhookOptions) (*WebhookEvent, error) {
	if err := VerifyWebhook(body, signatureHeader, secret); err != nil {
		return nil, err
	}
	var ev WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, err
	}
	ev.Raw = body
	if ev.Event == "" {
		ev.Event = ev.Type
	}
	if ev.Type == "" {
		ev.Type = ev.Event
	}
	if ev.DeliveryID == "" {
		ev.DeliveryID = ev.ID
	}
	if len(opts) > 0 && opts[0].Tolerance > 0 {
		now := time.Now
		if opts[0].Now != nil {
			now = opts[0].Now
		}
		if at, err := ev.OccurredTime(); err == nil && now().Sub(at) > opts[0].Tolerance {
			return nil, ErrWebhookTooOld
		}
	}
	return &ev, nil
}

// MaxWebhookBodyBytes caps how much of a delivery ParseWebhookRequest reads.
const MaxWebhookBodyBytes = 1 << 20

// ParseWebhookRequest reads, verifies and parses a webhook delivery from an
// incoming HTTP request.
func ParseWebhookRequest(r *http.Request, secret string, opts ...ParseWebhookOptions) (*WebhookEvent, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxWebhookBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxWebhookBodyBytes {
		return nil, errors.New("miosa: webhook body too large")
	}
	ev, err := ParseWebhook(body, r.Header.Get(WebhookSignatureHeader), secret, opts...)
	if err != nil {
		return nil, err
	}
	// The headers repeat the body's event and delivery ids; keep them when the
	// body omitted them.
	if ev.Event == "" {
		ev.Event = r.Header.Get(WebhookEventHeader)
		ev.Type = ev.Event
	}
	if ev.DeliveryID == "" {
		ev.DeliveryID = r.Header.Get(WebhookDeliveryHeader)
		ev.ID = ev.DeliveryID
	}
	return ev, nil
}

// WebhookHandler returns an http.Handler that verifies each delivery and calls
// fn. A bad or missing signature is answered 401 and never reaches fn; a
// malformed body is 400; fn returning an error is 500, so MIOSA retries; nil is
// 204.
func WebhookHandler(secret string, fn func(ctx context.Context, ev *WebhookEvent) error, opts ...ParseWebhookOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ev, err := ParseWebhookRequest(r, secret, opts...)
		switch {
		case err == nil:
		case errors.Is(err, ErrWebhookSignatureMissing), errors.Is(err, ErrWebhookSignatureInvalid),
			errors.Is(err, ErrWebhookSignatureMismatch), errors.Is(err, ErrWebhookTooOld):
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		default:
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := fn(r.Context(), ev); err != nil {
			http.Error(w, "handler error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
