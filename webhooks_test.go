package miosa_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// ─── Verification ────────────────────────────────────────────────────────────

const (
	whBody   = `{"id":"d1","event":"sandbox.created","data":{"id":"sbx_1"}}`
	whSecret = "whsec_test_secret"
	// Computed independently with Python's hmac module.
	whSig = "sha256=8b2235cca7267651fabc07d21aad1bab5fe09e99f536df5879e5734063c8d377"
)

func TestSignWebhookMatchesAnIndependentImplementation(t *testing.T) {
	if got := miosa.SignWebhook([]byte(whBody), whSecret); got != whSig {
		t.Fatalf("SignWebhook = %s, want %s", got, whSig)
	}
}

func TestVerifyWebhook(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		header string
		secret string
		want   error
	}{
		{"valid", whBody, whSig, whSecret, nil},
		{"valid with padding whitespace", whBody, "  " + whSig + " ", whSecret, nil},
		{"missing", whBody, "", whSecret, miosa.ErrWebhookSignatureMissing},
		{"no prefix", whBody, strings.TrimPrefix(whSig, "sha256="), whSecret, miosa.ErrWebhookSignatureInvalid},
		{"wrong algorithm", whBody, "sha1=abcd", whSecret, miosa.ErrWebhookSignatureInvalid},
		{"not hex", whBody, "sha256=zzzz", whSecret, miosa.ErrWebhookSignatureInvalid},
		{"short digest", whBody, "sha256=abcd", whSecret, miosa.ErrWebhookSignatureInvalid},
		{"tampered body", whBody + " ", whSig, whSecret, miosa.ErrWebhookSignatureMismatch},
		{"wrong secret", whBody, whSig, "whsec_other", miosa.ErrWebhookSignatureMismatch},
		{"empty secret never validates a real signature", whBody, whSig, "", miosa.ErrWebhookSignatureMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := miosa.VerifyWebhook([]byte(tc.body), tc.header, tc.secret)
			if !errors.Is(err, tc.want) && !(err == nil && tc.want == nil) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseWebhook(t *testing.T) {
	body := `{"id":"d1","delivery_id":"d1","event":"sandbox.created","type":"sandbox.created","occurred_at":"2026-10-10T00:00:00Z","timestamp":"2026-10-10T00:00:00Z","tenant_id":"t1","data":{"id":"sbx_1","state":"running"}}`
	sig := miosa.SignWebhook([]byte(body), whSecret)

	ev, err := miosa.ParseWebhook([]byte(body), sig, whSecret)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "d1" || ev.DeliveryID != "d1" || ev.Event != "sandbox.created" || ev.Type != "sandbox.created" || ev.TenantID != "t1" {
		t.Errorf("event = %+v", ev)
	}
	var data struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := ev.Decode(&data); err != nil || data.ID != "sbx_1" || data.State != "running" {
		t.Errorf("data = %+v, %v", data, err)
	}
	if at, err := ev.OccurredTime(); err != nil || at.Year() != 2026 {
		t.Errorf("OccurredTime = %v, %v", at, err)
	}
	if string(ev.Raw) != body {
		t.Error("Raw must be the verified body")
	}

	if _, err := miosa.ParseWebhook([]byte(body), "sha256=00", whSecret); err == nil {
		t.Error("an invalid signature parsed")
	}
	bad := `not json`
	if _, err := miosa.ParseWebhook([]byte(bad), miosa.SignWebhook([]byte(bad), whSecret), whSecret); err == nil {
		t.Error("a signed non-JSON body parsed")
	}
}

func TestParseWebhookFillsMissingNamesAndEnforcesTolerance(t *testing.T) {
	body := `{"type":"sandbox.deleted","id":"d9","occurred_at":"2026-10-10T00:00:00Z","data":{}}`
	sig := miosa.SignWebhook([]byte(body), whSecret)
	at := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

	ev, err := miosa.ParseWebhook([]byte(body), sig, whSecret, miosa.ParseWebhookOptions{Tolerance: 5 * time.Minute, Now: func() time.Time { return at.Add(time.Minute) }})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event != "sandbox.deleted" || ev.DeliveryID != "d9" {
		t.Errorf("event = %+v", ev)
	}
	_, err = miosa.ParseWebhook([]byte(body), sig, whSecret, miosa.ParseWebhookOptions{Tolerance: 5 * time.Minute, Now: func() time.Time { return at.Add(time.Hour) }})
	if !errors.Is(err, miosa.ErrWebhookTooOld) {
		t.Errorf("err = %v", err)
	}
	// No tolerance: late retries and replays are accepted.
	if _, err := miosa.ParseWebhook([]byte(body), sig, whSecret, miosa.ParseWebhookOptions{Now: func() time.Time { return at.Add(24 * time.Hour) }}); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestWebhookHandler(t *testing.T) {
	var got *miosa.WebhookEvent
	handler := miosa.WebhookHandler(whSecret, func(ctx context.Context, ev *miosa.WebhookEvent) error {
		got = ev
		if ev.Event == "boom" {
			return errors.New("downstream down")
		}
		return nil
	})
	post := func(body, sig string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/hooks", strings.NewReader(body))
		if sig != "" {
			req.Header.Set(miosa.WebhookSignatureHeader, sig)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(whBody, whSig); rec.Code != http.StatusNoContent || got == nil || got.Event != "sandbox.created" {
		t.Errorf("valid delivery: %d, event %+v", rec.Code, got)
	}
	got = nil
	if rec := post(whBody, "sha256="+strings.Repeat("0", 64)); rec.Code != http.StatusUnauthorized || got != nil {
		t.Errorf("a forged signature must be 401 and never reach the handler: %d %+v", rec.Code, got)
	}
	if rec := post(whBody, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("a missing signature: %d", rec.Code)
	}
	notJSON := "{"
	if rec := post(notJSON, miosa.SignWebhook([]byte(notJSON), whSecret)); rec.Code != http.StatusBadRequest {
		t.Errorf("a signed malformed body: %d", rec.Code)
	}
	boom := `{"id":"d2","event":"boom","data":{}}`
	if rec := post(boom, miosa.SignWebhook([]byte(boom), whSecret)); rec.Code != http.StatusInternalServerError {
		t.Errorf("a failing handler must answer 500 so MIOSA retries: %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/hooks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Errorf("GET: %d %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestParseWebhookRequestUsesHeadersAndCapsTheBody(t *testing.T) {
	body := `{"data":{}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(miosa.WebhookSignatureHeader, miosa.SignWebhook([]byte(body), whSecret))
	req.Header.Set(miosa.WebhookEventHeader, "deployment.succeeded")
	req.Header.Set(miosa.WebhookDeliveryHeader, "del-7")

	ev, err := miosa.ParseWebhookRequest(req, whSecret)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event != "deployment.succeeded" || ev.Type != "deployment.succeeded" || ev.DeliveryID != "del-7" || ev.ID != "del-7" {
		t.Errorf("event = %+v", ev)
	}

	huge := bytes.Repeat([]byte("a"), miosa.MaxWebhookBodyBytes+10)
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(huge))
	req.Header.Set(miosa.WebhookSignatureHeader, miosa.SignWebhook(huge, whSecret))
	if _, err := miosa.ParseWebhookRequest(req, whSecret); err == nil {
		t.Error("an oversize body was read")
	}
}

// ─── Management ──────────────────────────────────────────────────────────────

var webhookJSON = map[string]interface{}{
	"id": "wh_1", "tenant_id": "t1", "workspace_id": "ws_1", "name": "ci", "description": "d", "url": "https://example.com/hook",
	"secret_preview": "whsec_••••••", "events": []string{"sandbox.created"}, "state": "active", "headers": map[string]string{"X-Env": "prod"},
	"retry_count": 5, "failure_count": 1, "last_triggered_at": "2026-10-10T00:00:00Z", "last_status_code": 200,
	"metadata": map[string]interface{}{"team": "a"}, "created_at": "x", "updated_at": "y",
}

func TestWebhooksCreateAndDecodeNewFields(t *testing.T) {
	created := map[string]interface{}{}
	for k, v := range webhookJSON {
		created[k] = v
	}
	delete(created, "secret_preview")
	created["secret"] = "whsec_realvalue"
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": created})
	retries := 5

	wh, err := client.Webhooks.Create(context.Background(), miosa.CreateWebhookInput{
		URL: "https://example.com/hook", Events: []string{"sandbox.created"}, WorkspaceID: "ws_1", Name: "ci", Description: "d",
		Headers: map[string]string{"X-Env": "prod"}, RetryCount: &retries, Metadata: map[string]interface{}{"team": "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/webhooks")
	body := got.Body(t)
	if body["workspace_id"] != "ws_1" || body["name"] != "ci" || body["retry_count"] != float64(5) || body["headers"].(map[string]interface{})["X-Env"] != "prod" {
		t.Errorf("body = %v", body)
	}
	if wh.Secret != "whsec_realvalue" || wh.State != "active" || !wh.IsActive() || wh.RetryCount != 5 || *wh.LastStatusCode != 200 || wh.Headers["X-Env"] != "prod" || wh.WorkspaceID != "ws_1" {
		t.Errorf("webhook = %+v", wh)
	}
}

func TestWebhookIsActiveFromStateOrFlag(t *testing.T) {
	if (miosa.WebhookData{State: "failed", Active: true}).IsActive() {
		t.Error("an explicit non-active state wins")
	}
	if !(miosa.WebhookData{Active: true}).IsActive() {
		t.Error("the legacy flag still works")
	}
}

func TestWebhooksUpdateSecretRotationAndState(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": webhookJSON})
	name := "renamed"
	rc := 0

	if _, err := client.Webhooks.Update(context.Background(), "wh_1", miosa.UpdateWebhookInput{Secret: "whsec_new", Name: &name, RetryCount: &rc, State: "inactive"}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/webhooks/wh_1")
	body := got.Body(t)
	if body["secret"] != "whsec_new" || body["name"] != "renamed" || body["retry_count"] != float64(0) || body["state"] != "inactive" {
		t.Errorf("body = %v (a retry_count of 0 must be sent)", body)
	}
}

func TestWebhooksEventsCatalog(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data": map[string]interface{}{"sandboxes": []string{"sandbox.created"}}, "events": []string{"sandbox.created", "deployment.succeeded"},
		"workspace_scoped_categories": []string{"sandboxes"},
		"signing":                     map[string]interface{}{"header": "x-miosa-signature", "algorithm": "hmac-sha256", "format": "sha256=<hex>"},
		"delivery_headers":            []string{"x-miosa-event", "x-miosa-delivery", "x-miosa-signature"},
	})

	cat, err := client.Webhooks.Events(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/webhooks/events")
	if len(cat.Events) != 2 || cat.Signing.Header != "x-miosa-signature" || cat.Signing.Format != "sha256=<hex>" || cat.WorkspaceScopedCategories[0] != "sandboxes" || len(cat.DeliveryHeaders) != 3 {
		t.Errorf("catalog = %+v", cat)
	}
	if !strings.EqualFold(cat.Signing.Header, miosa.WebhookSignatureHeader) {
		t.Error("the SDK verifies the header the catalog advertises")
	}
}

func TestWebhooksDeliveriesAndReplay(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"data": map[string]interface{}{
				"id": "del_new", "webhook_id": "wh_1", "replay_of": "del_old", "event_type": "sandbox.created", "state": "pending", "attempts": 0,
			}})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{
			"id": "del_old", "webhook_id": "wh_1", "event_type": "sandbox.created", "state": "dead_letter", "response_status": 500,
			"response_body": "boom", "attempts": 4, "next_retry_at": nil, "delivered_at": nil, "created_at": "2026-10-10T00:00:00Z",
		}}})
	})

	list, err := client.Webhooks.DeliveriesWith(context.Background(), "wh_1", miosa.ListDeliveriesOptions{Limit: 10, Before: "2026-10-11T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/webhooks/wh_1/deliveries")
	if got.RawQuery != "before=2026-10-11T00%3A00%3A00Z&limit=10" {
		t.Errorf("query = %q", got.RawQuery)
	}
	d := list.Data[0]
	if d.State != "dead_letter" || *d.ResponseStatus != 500 || d.Attempts != 4 || d.CreatedAt == "" {
		t.Errorf("delivery = %+v", d)
	}

	re, err := client.Webhooks.Replay(context.Background(), "wh_1", "del_old", "")
	if err != nil {
		t.Fatal(err)
	}
	got = rec.Last(t)
	assertReq(t, got, "POST", "/webhooks/wh_1/deliveries/del_old/replay")
	id, _ := got.Body(t)["replay_id"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Errorf("a generated replay_id must be a v4 UUID, got %q", id)
	}
	if re.ID != "del_new" || re.ReplayOf != "del_old" || re.State != "pending" {
		t.Errorf("replay = %+v", re)
	}

	if _, err := client.Webhooks.Replay(context.Background(), "wh_1", "del_old", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if rec.Last(t).Body(t)["replay_id"] != "11111111-1111-4111-8111-111111111111" {
		t.Error("a caller-supplied replay id must be used verbatim")
	}
}

func TestWebhookReplayConflicts(t *testing.T) {
	client, _ := newFixedClient(t, 409, map[string]interface{}{"error": "Replay ID already used"})
	_, err := client.Webhooks.Replay(context.Background(), "wh_1", "del_old", "11111111-1111-4111-8111-111111111111")
	m := miosa.AsMiosaError(err)
	if m == nil || m.StatusCode != 409 || m.Message != "Replay ID already used" {
		t.Fatalf("error = %#v", err)
	}
}

// ─── Event subscriptions ─────────────────────────────────────────────────────

func TestEventSubscriptionsLifecycle(t *testing.T) {
	sub := map[string]interface{}{"id": "sub_1", "consumer_kind": "workflow", "consumer_id": "wf_1", "event_type": "sandbox.created",
		"filter": map[string]interface{}{"tags": "prod"}, "workspace_id": "ws_1", "active": true, "inserted_at": "x"}
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/event-subscriptions":
			writeJSON(w, 200, map[string]interface{}{"data": []interface{}{sub}, "total": 1})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet:
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "dl_1", "event_id": "ev_1", "state": "failed", "attempts": 3, "next_attempt_at": "2026-10-10T01:00:00Z", "response_summary": "HTTP 500"}}, "next_cursor": nil})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/event-deliveries/"):
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"data": map[string]interface{}{"id": "dl_1", "state": "pending"}})
		case r.Method == http.MethodPost:
			writeJSON(w, 201, map[string]interface{}{"data": sub})
		default:
			writeJSON(w, 200, map[string]interface{}{"data": sub})
		}
	})
	ctx := context.Background()

	list, err := client.Events.List(ctx, miosa.ListEventSubscriptionsOptions{ConsumerKind: "workflow", ConsumerID: "wf_1"})
	if err != nil || len(list) != 1 || list[0].EventType != "sandbox.created" || list[0].Filter["tags"] != "prod" {
		t.Fatalf("%+v %v", list, err)
	}
	if got := rec.Last(t).RawQuery; got != "consumer_id=wf_1&consumer_kind=workflow" {
		t.Errorf("query = %q", got)
	}

	if _, err := client.Events.Create(ctx, miosa.CreateEventSubscriptionInput{ConsumerKind: "workflow", ConsumerID: "wf_1", EventType: "sandbox.created", WorkspaceID: "ws_1"}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/event-subscriptions")
	if _, err := client.Events.Create(ctx, miosa.CreateEventSubscriptionInput{}); err == nil {
		t.Error("an empty subscription was accepted")
	}

	off := false
	if _, err := client.Events.Update(ctx, "sub_1", miosa.UpdateEventSubscriptionInput{Active: &off}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/event-subscriptions/sub_1")
	if got.Body(t)["active"] != false {
		t.Errorf("body = %v", got.Body(t))
	}

	dl, err := client.Events.Deliveries(ctx, "sub_1")
	if err != nil || dl[0].State != "failed" || dl[0].Attempts != 3 || dl[0].ResponseSummary != "HTTP 500" {
		t.Fatalf("%+v %v", dl, err)
	}
	assertReq(t, rec.Last(t), "GET", "/event-subscriptions/sub_1/deliveries")

	re, err := client.Events.Redeliver(ctx, "dl_1")
	if err != nil || re.State != "pending" {
		t.Fatalf("%+v %v", re, err)
	}
	assertReq(t, rec.Last(t), "POST", "/event-deliveries/dl_1/redeliver")

	if err := client.Events.Delete(ctx, "sub_1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/event-subscriptions/sub_1")
}

func TestEventSubscriptionErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{{422, "UNKNOWN_EVENT_TYPE"}, {422, "INVALID_FILTER"}, {404, "CONSUMER_NOT_FOUND"}, {404, "SUBSCRIPTION_NOT_FOUND"}} {
		client, _ := newFixedClient(t, tc.status, map[string]interface{}{"error": map[string]interface{}{"code": tc.code}})
		_, err := client.Events.Create(context.Background(), miosa.CreateEventSubscriptionInput{ConsumerKind: "workflow", ConsumerID: "x", EventType: "y"})
		if !miosa.IsCode(err, tc.code) {
			t.Errorf("%s: error = %v", tc.code, err)
		}
	}
	client, _ := newFixedClient(t, 409, map[string]interface{}{"error": map[string]interface{}{"code": "NOT_REDELIVERABLE"}})
	if _, err := client.Events.Redeliver(context.Background(), "dl"); !miosa.IsCode(err, "NOT_REDELIVERABLE") {
		t.Errorf("error = %v", err)
	}
}

func TestEventOutbox(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data":        []map[string]interface{}{{"seq": 41, "type": "sandbox.created"}, {"seq": 42, "type": "sandbox.deleted"}},
		"next_cursor": "42",
	})

	page, err := client.Events.Outbox(context.Background(), miosa.PlatformEventsOptions{WorkspaceID: "ws_1", Type: "sandbox.created", AfterSeq: "40", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/platform-events")
	if got.RawQuery != "after_seq=40&limit=100&type=sandbox.created&workspace_id=ws_1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if len(page.Data) != 2 || page.Data[1]["type"] != "sandbox.deleted" || page.NextCursor != "42" {
		t.Errorf("page = %+v", page)
	}

	client, rec = newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"stats": map[string]interface{}{"pending": 3}, "max_attempts": 8}})
	health, err := client.Events.OutboxHealth(context.Background())
	if err != nil || health["max_attempts"] != float64(8) {
		t.Fatalf("%+v %v", health, err)
	}
	assertReq(t, rec.Last(t), "GET", "/platform-events/outbox")
}
