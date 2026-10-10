package miosa_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var billToJSON = map[string]interface{}{"data": map[string]interface{}{
	"billing":        map[string]interface{}{"id": "o1", "name": "Acme", "slug": "acme", "type": "company", "role": "member"},
	"source":         "setting",
	"setting_pinned": true, "setting_stale": false, "viewing_id": "o2", "can_change": true,
	"options": []map[string]interface{}{
		{"id": "o2", "name": "Mine", "slug": "mine", "type": "personal", "role": "owner", "viewing": true, "billing": false, "pinned": false},
		{"id": "o1", "name": "Acme", "slug": "acme", "type": "company", "role": "member", "viewing": false, "billing": true, "pinned": true},
	},
}}

func TestBillToGet(t *testing.T) {
	client, rec := newFixedClient(t, 200, billToJSON)

	b, err := client.BillTo.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/bill-to")
	if b.Billing.Slug != "acme" || b.Source != miosa.BillToSourceSetting || !b.SettingPinned || b.SettingStale || !b.CanChange {
		t.Errorf("bill-to = %+v", b)
	}
	if len(b.Options) != 2 || !b.Options[0].Viewing || !b.Options[1].Billing || !b.Options[1].Pinned || b.Options[1].Role != "member" {
		t.Errorf("options = %+v", b.Options)
	}
}

func TestBillToSetAndClear(t *testing.T) {
	client, rec := newFixedClient(t, 200, billToJSON)

	if _, err := client.BillTo.Set(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/bill-to")
	if got.Body(t)["org"] != "acme" {
		t.Errorf("body = %v", got.Body(t))
	}

	if err := client.BillTo.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/bill-to")

	if _, err := client.BillTo.Set(context.Background(), ""); err == nil {
		t.Error("empty org accepted")
	}
}

func TestBillToLockedCredential(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusForbidden, errorEnvelope("BILL_TO_LOCKED", "this credential is locked to its organization", nil))

	_, err := client.BillTo.Set(context.Background(), "acme")

	var perm *miosa.PermissionError
	if !errors.As(err, &perm) || perm.Code != miosa.CodeBillToLocked {
		t.Fatalf("error = %#v", err)
	}
}

func TestBillToOverrideHeaderOnContextAndClient(t *testing.T) {
	t.Run("context", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, billToJSON)
		ctx := miosa.WithBillToContext(context.Background(), "acme")
		if _, err := client.BillTo.Get(ctx); err != nil {
			t.Fatal(err)
		}
		if got := rec.Last(t).Header.Get(miosa.HeaderBillTo); got != "acme" {
			t.Errorf("%s = %q", miosa.HeaderBillTo, got)
		}
		if _, err := client.BillTo.Get(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := rec.Last(t).Header.Get(miosa.HeaderBillTo); got != "" {
			t.Errorf("a plain context must not carry the override, got %q", got)
		}
	})
	t.Run("client option", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, billToJSON, miosa.WithBillTo("beta"))
		if _, err := client.BillTo.Get(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := rec.Last(t).Header.Get(miosa.HeaderBillTo); got != "beta" {
			t.Errorf("%s = %q", miosa.HeaderBillTo, got)
		}
	})
	t.Run("context beats client", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, billToJSON, miosa.WithBillTo("beta"))
		if _, err := client.BillTo.Get(miosa.WithBillToContext(context.Background(), "gamma")); err != nil {
			t.Fatal(err)
		}
		if got := rec.Last(t).Header.Get(miosa.HeaderBillTo); got != "gamma" {
			t.Errorf("%s = %q", miosa.HeaderBillTo, got)
		}
	})
	t.Run("sandbox create carries it", func(t *testing.T) {
		client, rec := newFixedClient(t, 201, sandboxEnvelope(nil))
		if _, err := client.Sandboxes.Create(miosa.WithBillToContext(context.Background(), "acme"), miosa.CreateSandboxInput{}); err != nil {
			t.Fatal(err)
		}
		if got := rec.Last(t).Header.Get(miosa.HeaderBillTo); got != "acme" {
			t.Errorf("%s = %q", miosa.HeaderBillTo, got)
		}
	})
}

func TestWithRequestHeadersMerges(t *testing.T) {
	client, rec := newFixedClient(t, 200, billToJSON, miosa.WithDefaultHeader("X-Trace", "t1"))
	ctx := miosa.WithRequestHeaders(context.Background(), map[string]string{"X-A": "1"})
	ctx = miosa.WithRequestHeaders(ctx, map[string]string{"X-B": "2"})
	if _, err := client.BillTo.Get(ctx); err != nil {
		t.Fatal(err)
	}
	h := rec.Last(t).Header
	if h.Get("X-A") != "1" || h.Get("X-B") != "2" || h.Get("X-Trace") != "t1" {
		t.Errorf("headers = %v", h)
	}
}

func TestLimits(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"organization":    map[string]interface{}{"id": "o1", "name": "Acme", "type": "company", "role": "member"},
		"source":          "request",
		"can_start":       false,
		"blocked_reasons": []string{"member_usage_cap_reached"},
		"credits":         map[string]interface{}{"balance_cents": 900, "available_cents": 899},
		"plan":            map[string]interface{}{"name": "pro"},
		"concurrency":     map[string]interface{}{"limit": 10, "running": 4, "remaining": 6},
		"spend":           map[string]interface{}{"mode": "limited", "cap_cents": 5000, "accounted_cents": 1200, "status": "active"},
		"member":          map[string]interface{}{"usage_cap_cents": 500, "usage_cents": 500, "max_concurrent_sandboxes": 2, "running_sandboxes": 1},
	}})

	l, err := client.BillTo.Limits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/limits")
	if l.CanStart || l.BlockedReasons[0] != miosa.BlockedMemberUsageCapReached || l.Source != miosa.BillToSourceRequest {
		t.Errorf("limits = %+v", l)
	}
	if l.Credits.AvailableCents != 899 || l.Plan.Name != "pro" || *l.Concurrency.Limit != 10 || *l.Concurrency.Remaining != 6 {
		t.Errorf("limits = %+v", l)
	}
	if *l.Spend.CapCents != 5000 || l.Spend.Status != "active" || l.Member == nil || *l.Member.MaxConcurrentSandboxes != 2 || l.Member.UsageCents != 500 {
		t.Errorf("spend/member = %+v %+v", l.Spend, l.Member)
	}
}

func TestLimitsUnlimitedPlanAndNoMemberCaps(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"can_start": true, "blocked_reasons": []string{},
		"concurrency": map[string]interface{}{"limit": nil, "running": 0, "remaining": nil},
		"spend":       map[string]interface{}{"mode": "unlimited", "cap_cents": nil},
		"member":      nil,
	}})
	l, err := client.BillTo.Limits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !l.CanStart || l.Concurrency.Limit != nil || l.Concurrency.Remaining != nil || l.Spend.CapCents != nil || l.Member != nil {
		t.Errorf("limits = %+v", l)
	}
}

func TestMemberCapsList(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"members": []map[string]interface{}{{
			"user_id": "u1", "email": "a@b.co", "name": "Ada", "role": "member",
			"usage_cap_cents": 500, "max_concurrent_sandboxes": nil, "usage_cents": 120, "running_sandboxes": 1,
		}},
		"window_start": "2026-10-01T00:00:00Z", "window_end": "2026-11-01T00:00:00Z",
	}})

	l, err := client.MemberCaps.List(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/tenants/t1/member-caps")
	m := l.Members[0]
	if m.Email != "a@b.co" || *m.UsageCapCents != 500 || m.MaxConcurrentSandboxes != nil || m.UsageCents != 120 || l.WindowEnd == "" {
		t.Errorf("list = %+v", l)
	}
}

func TestMemberCapsSetTriState(t *testing.T) {
	cap, conc := int64(500), 3
	cases := []struct {
		name string
		in   miosa.MemberCapsInput
		want map[string]interface{}
	}{
		{"set both", miosa.MemberCapsInput{UsageCapCents: &cap, MaxConcurrentSandboxes: &conc},
			map[string]interface{}{"usage_cap_cents": float64(500), "max_concurrent_sandboxes": float64(3)}},
		{"clear usage keeps concurrency untouched", miosa.MemberCapsInput{ClearUsageCap: true},
			map[string]interface{}{"usage_cap_cents": nil}},
		{"set one clear other", miosa.MemberCapsInput{MaxConcurrentSandboxes: &conc, ClearUsageCap: true},
			map[string]interface{}{"usage_cap_cents": nil, "max_concurrent_sandboxes": float64(3)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"user_id": "u1", "role": "member"}})
			if _, err := client.MemberCaps.Set(context.Background(), "t1", "u1", tc.in); err != nil {
				t.Fatal(err)
			}
			got := rec.Last(t)
			assertReq(t, got, "PUT", "/tenants/t1/member-caps/u1")
			body := got.Body(t)
			if len(body) != len(tc.want) {
				t.Errorf("body = %v, want %v", body, tc.want)
			}
			for k, v := range tc.want {
				if got, present := body[k]; !present || got != v {
					t.Errorf("body[%s] = %v (present=%v), want %v", k, got, present, v)
				}
			}
		})
	}
}

func TestMemberCapsSetRequiresAtLeastOneCap(t *testing.T) {
	client, rec := newFixedClient(t, 200, nil)
	if _, err := client.MemberCaps.Set(context.Background(), "t1", "u1", miosa.MemberCapsInput{}); err == nil {
		t.Fatal("empty caps accepted")
	}
	if rec.Count() != 0 {
		t.Error("must not reach the server")
	}
}

func TestMemberCapsClear(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"user_id": "u1", "role": "member", "usage_cap_cents": nil}})
	m, err := client.MemberCaps.Clear(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/tenants/t1/member-caps/u1")
	if m.UsageCapCents != nil {
		t.Errorf("member = %+v", m)
	}
}

func TestMemberCapErrorsAreTypedWithCodes(t *testing.T) {
	t.Run("usage cap is 402", func(t *testing.T) {
		client, _ := newFixedClient(t, 402, errorEnvelope("TEAM_MEMBER_CAP_REACHED", "usage cap reached", nil))
		_, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{})
		var credits *miosa.InsufficientCreditsError
		if !errors.As(err, &credits) || credits.Code != miosa.CodeTeamMemberCapReached {
			t.Fatalf("error = %#v", err)
		}
	})
	t.Run("concurrency cap is 429", func(t *testing.T) {
		client, _ := newFixedClient(t, 429, errorEnvelope("MEMBER_LIMIT_REACHED", "concurrency cap reached", nil))
		_, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{})
		var rl *miosa.RateLimitError
		if !errors.As(err, &rl) || rl.Code != miosa.CodeMemberLimitReached {
			t.Fatalf("error = %#v", err)
		}
	})
}
