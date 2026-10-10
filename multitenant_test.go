package miosa_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"github.com/gorilla/websocket"
)

// ─── Scoped tokens ───────────────────────────────────────────────────────────

func TestScopedTokenMint(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{
		"id": "tok_1", "token": "eyJ.scoped.token", "expires_at": "2026-10-10T12:00:00Z", "scopes": []string{"sandboxes:read", "sandboxes:exec"},
	})

	tok, err := client.ScopedTokens.Mint(context.Background(), miosa.MintScopedTokenInput{
		UserID: "dr-smith-456", WorkspaceID: "ws_1", Scopes: []string{"sandboxes:read", "sandboxes:exec"}, ExpiresInSeconds: 900,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/tokens/scoped")
	body := got.Body(t)
	if body["user_id"] != "dr-smith-456" || body["workspace_id"] != "ws_1" || body["expires_in_seconds"] != float64(900) {
		t.Errorf("body = %v", body)
	}
	if tok.ID != "tok_1" || tok.Token != "eyJ.scoped.token" || len(tok.Scopes) != 2 || tok.ExpiresAt == "" {
		t.Errorf("token = %+v", tok)
	}
}

func TestScopedTokenMintValidationAndRefusals(t *testing.T) {
	client, rec := newFixedClient(t, 201, nil)
	cases := []miosa.MintScopedTokenInput{
		{WorkspaceID: "ws"},
		{UserID: "u"},
		{UserID: "u", WorkspaceID: "ws", ExpiresInSeconds: miosa.MaxScopedTokenTTLSeconds + 1},
		{UserID: "u", WorkspaceID: "ws", ExpiresInSeconds: -1},
	}
	for i, in := range cases {
		if _, err := client.ScopedTokens.Mint(context.Background(), in); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if rec.Count() != 0 {
		t.Error("invalid input reached the server")
	}

	client, _ = newFixedClient(t, 403, map[string]interface{}{"error": "Requested scopes exceed caller's permissions", "invalid_scopes": []string{"admin:all"}})
	_, err := client.ScopedTokens.Mint(context.Background(), miosa.MintScopedTokenInput{UserID: "u", WorkspaceID: "ws", Scopes: []string{"admin:all"}})
	var perm *miosa.PermissionError
	if !errors.As(err, &perm) || perm.Message != "Requested scopes exceed caller's permissions" {
		t.Errorf("error = %#v", err)
	}
}

func TestScopedTokenListPagesAndRevoke(t *testing.T) {
	pages := 0
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			writeJSON(w, 200, map[string]interface{}{"revoked": true})
			return
		}
		pages++
		if r.URL.Query().Get("before") == "" {
			writeJSON(w, 200, map[string]interface{}{
				"data":        []map[string]interface{}{{"id": "t2", "user_id": "u", "workspace_id": "ws", "scopes": []string{"a"}, "expires_at": "2099-01-01T00:00:00Z", "created_at": "x"}},
				"next_cursor": "t2",
			})
			return
		}
		writeJSON(w, 200, map[string]interface{}{
			"data":        []map[string]interface{}{{"id": "t1", "user_id": "u", "workspace_id": "ws", "revoked_at": "2026-01-01T00:00:00Z", "expires_at": "2099-01-01T00:00:00Z"}},
			"next_cursor": nil,
		})
	})

	page, err := client.ScopedTokens.List(context.Background(), "")
	if err != nil || page.NextCursor != "t2" || page.Data[0].UserID != "u" {
		t.Fatalf("page = %+v, %v", page, err)
	}
	all, err := client.ScopedTokens.All(context.Background())
	if err != nil || len(all) != 2 || all[1].ID != "t1" || pages != 3 {
		t.Fatalf("all = %+v, %v (pages %d)", all, err, pages)
	}
	if !all[0].Active(time.Now()) || all[1].Active(time.Now()) {
		t.Error("Active must reflect revocation")
	}

	if err := client.ScopedTokens.Revoke(context.Background(), "t2"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/tokens/scoped/t2")
	if err := client.ScopedTokens.Revoke(context.Background(), ""); err == nil {
		t.Error("empty id accepted")
	}
}

func TestScopedTokenActiveHandlesExpiryAndBadTimestamps(t *testing.T) {
	past := miosa.ScopedTokenInfo{ExpiresAt: "2020-01-01T00:00:00Z"}
	future := miosa.ScopedTokenInfo{ExpiresAt: "2999-01-01T00:00:00Z"}
	if past.Active(time.Now()) || !future.Active(time.Now()) {
		t.Error("expiry not honored")
	}
	if !(miosa.ScopedTokenInfo{ExpiresAt: "garbage"}).Active(time.Now()) {
		t.Error("an unreadable expiry must not hide a token that is not revoked")
	}
}

func TestAsUserAuthenticatesAsTheTokenAndSharesTheTransport(t *testing.T) {
	admin, rec := newFixedClient(t, 200, map[string]interface{}{"auth": map[string]interface{}{"method": "scoped"}}, miosa.WithTenant("org-1"))
	user := admin.AsUser("eyJ.user.token")

	if _, err := user.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := rec.Last(t).Header
	if h.Get("Authorization") != "Bearer eyJ.user.token" {
		t.Errorf("Authorization = %q", h.Get("Authorization"))
	}
	if h.Get(miosa.HeaderTenant) != "org-1" {
		t.Errorf("the derived client keeps the tenant, got %q", h.Get(miosa.HeaderTenant))
	}
	if _, err := admin.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t).Header.Get("Authorization"); got != "Bearer msk_u_test" {
		t.Errorf("deriving must not change the parent, got %q", got)
	}
}

func TestDerivedClientsDoNotShareMutableHeaders(t *testing.T) {
	parent, rec := newFixedClient(t, 200, map[string]interface{}{}, miosa.WithDefaultHeader("X-A", "1"))
	child := parent.With(miosa.WithDefaultHeader("X-B", "2"))

	if _, err := parent.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rec.Last(t).Header.Get("X-B") != "" {
		t.Error("a child's header leaked into its parent")
	}
	if _, err := child.Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h := rec.Last(t).Header; h.Get("X-A") != "1" || h.Get("X-B") != "2" {
		t.Errorf("child headers = %v", h)
	}
}

func TestForTenantAndUserAgentOverride(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{}, miosa.WithUserAgentOverride("acme-platform/3.1"))
	if _, err := client.ForTenant("org-7").Whoami(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := rec.Last(t).Header
	if h.Get(miosa.HeaderTenant) != "org-7" {
		t.Errorf("tenant = %q", h.Get(miosa.HeaderTenant))
	}
	if h.Get("User-Agent") != "acme-platform/3.1" || strings.Contains(h.Get("User-Agent"), "miosa-go") {
		t.Errorf("User-Agent = %q; an override must replace the SDK identity", h.Get("User-Agent"))
	}
}

// ─── Policies ────────────────────────────────────────────────────────────────

func TestPoliciesTenantFlatShape(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"quotas":      map[string]interface{}{"max_sandboxes": 10, "max_credit_cents": 5000},
		"permissions": map[string]interface{}{"can_use_gpu": true},
	})

	p, err := client.Policies.Tenant(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/tenant/policy")
	if *p.Quotas.MaxSandboxes != 10 || *p.Quotas.MaxCreditCents != 5000 || !*p.Permissions.CanUseGPU || p.Features != nil {
		t.Errorf("policy = %+v", p)
	}
}

func TestPoliciesEmptyAndWrappedShapes(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{})
	p, err := client.Policies.Tenant(context.Background())
	if err != nil || p.Quotas != nil || p.Permissions != nil {
		t.Fatalf("empty tenant policy = %+v, %v", p, err)
	}

	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"features": map[string]interface{}{"snapshot_enabled": false}}})
	p, err = client.Policies.Workspace(context.Background(), "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/workspaces/ws_1/policy")
	if p.Features == nil || *p.Features.SnapshotEnabled {
		t.Errorf("policy = %+v", p)
	}
}

func TestPoliciesSetSendsOnlyWhatChanged(t *testing.T) {
	yes, no := true, false
	max := 3
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"quotas": map[string]interface{}{"max_sandboxes": 3}}})

	if _, err := client.Policies.SetExternalUser(context.Background(), "dr-smith", miosa.Policy{
		Quotas:      &miosa.PolicyQuotas{MaxSandboxes: &max},
		Permissions: &miosa.PolicyPermissions{CanCreateSandbox: &yes, CanUseGPU: &no},
	}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PUT", "/external-users/dr-smith/policy")
	body := got.Body(t)
	if len(body) != 2 {
		t.Errorf("body = %v", body)
	}
	if q := body["quotas"].(map[string]interface{}); len(q) != 1 || q["max_sandboxes"] != float64(3) {
		t.Errorf("quotas = %v", q)
	}
	if perms := body["permissions"].(map[string]interface{}); perms["can_use_gpu"] != false || perms["can_create_sandbox"] != true {
		t.Errorf("a false must be sent, not dropped: %v", perms)
	}
}

func TestPoliciesAllTiersRoutes(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{})
	ctx := context.Background()
	calls := []struct {
		name         string
		call         func() error
		method, path string
	}{
		{"set tenant", func() error { _, err := client.Policies.SetTenant(ctx, miosa.Policy{}); return err }, "PUT", "/tenant/policy"},
		{"delete tenant", func() error { return client.Policies.DeleteTenant(ctx) }, "DELETE", "/tenant/policy"},
		{"set workspace", func() error { _, err := client.Policies.SetWorkspace(ctx, "ws", miosa.Policy{}); return err }, "PUT", "/workspaces/ws/policy"},
		{"delete workspace", func() error { return client.Policies.DeleteWorkspace(ctx, "ws") }, "DELETE", "/workspaces/ws/policy"},
		{"get external", func() error { _, err := client.Policies.ExternalUser(ctx, "u1"); return err }, "GET", "/external-users/u1/policy"},
		{"delete external", func() error { return client.Policies.DeleteExternalUser(ctx, "u1") }, "DELETE", "/external-users/u1/policy"},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); err != nil {
				t.Fatal(err)
			}
			assertReq(t, rec.Last(t), c.method, c.path)
		})
	}
	if _, err := client.Policies.Workspace(ctx, ""); err == nil {
		t.Error("empty workspace accepted")
	}
	if _, err := client.Policies.ExternalUser(ctx, ""); err == nil {
		t.Error("empty external user accepted")
	}
	if err := client.Policies.DeleteExternalUser(ctx, ""); err == nil {
		t.Error("empty external user accepted")
	}
	if err := client.Policies.DeleteWorkspace(ctx, ""); err == nil {
		t.Error("empty workspace accepted")
	}
	if _, err := client.Policies.SetWorkspace(ctx, "", miosa.Policy{}); err == nil {
		t.Error("empty workspace accepted")
	}
	if _, err := client.Policies.SetExternalUser(ctx, "", miosa.Policy{}); err == nil {
		t.Error("empty external user accepted")
	}
}

func TestPoliciesForbiddenWithoutManagePermission(t *testing.T) {
	client, _ := newFixedClient(t, 403, map[string]interface{}{"error": "forbidden"})
	_, err := client.Policies.SetTenant(context.Background(), miosa.Policy{})
	var perm *miosa.PermissionError
	if !errors.As(err, &perm) {
		t.Fatalf("error = %#v", err)
	}
}

func TestEffectivePolicyCarriesSources(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"quotas":      map[string]interface{}{"value": map[string]interface{}{"max_sandboxes": 2, "max_concurrent": 1}, "source": "user"},
		"permissions": map[string]interface{}{"value": map[string]interface{}{"can_create_sandbox": true}, "source": "platform"},
		"billing":     map[string]interface{}{"value": map[string]interface{}{"enforce_credit_limit": true}, "source": "tenant"},
	}})

	eff, err := client.Policies.EffectiveForExternalUser(context.Background(), "dr-smith", "ws_1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/external-users/dr-smith/effective-policy")
	if got.RawQuery != "workspace_id=ws_1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	q, src, err := eff.Quotas()
	if err != nil || src != "user" || *q.MaxSandboxes != 2 || *q.MaxConcurrent != 1 {
		t.Errorf("quotas = %+v from %q (%v)", q, src, err)
	}
	perms, src, err := eff.Permissions()
	if err != nil || src != "platform" || !*perms.CanCreateSandbox {
		t.Errorf("permissions = %+v from %q (%v)", perms, src, err)
	}
	if eff["billing"].Source != "tenant" {
		t.Errorf("billing source = %q", eff["billing"].Source)
	}
	if _, err := client.Policies.EffectiveForExternalUser(context.Background(), "", ""); err == nil {
		t.Error("empty external user accepted")
	}
}

func TestEffectivePolicyMissingSectionsDecodeEmpty(t *testing.T) {
	var eff miosa.EffectivePolicy
	q, src, err := eff.Quotas()
	if err != nil || src != "" || q.MaxSandboxes != nil {
		t.Errorf("%+v %q %v", q, src, err)
	}
	p, src, err := eff.Permissions()
	if err != nil || src != "" || p.CanUseGPU != nil {
		t.Errorf("%+v %q %v", p, src, err)
	}
}

// ─── Quotas: the API's real shape ────────────────────────────────────────────

func TestQuotasDecodeTheNestedShapeTheAPISends(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"external_user_id": "dr-smith",
		"limits":           map[string]interface{}{"max_sandboxes": 5, "max_concurrent": 2, "max_storage_gb": nil, "max_credit_cents": 1000},
		"usage":            map[string]interface{}{"total_sandboxes": 3, "concurrent": 1},
	}})

	q, err := client.Quotas.Get(context.Background(), "dr-smith")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/quotas/external/dr-smith")
	if *q.Limits.MaxSandboxes != 5 || q.Limits.MaxStorageGB != nil || q.Usage.TotalSandboxes != 3 || q.Usage.Concurrent != 1 {
		t.Errorf("quota = %+v", q)
	}
	if q.MaxSandboxes == nil || *q.MaxSandboxes != 5 || q.CurrentSandboxes != 3 || *q.MaxCreditCents != 1000 {
		t.Errorf("the flat fields must be filled for existing callers: %+v", q)
	}
}

func TestQuotasStillDecodeTheFlatShape(t *testing.T) {
	client, _ := newFixedClient(t, 200, map[string]interface{}{"external_user_id": "u", "max_sandboxes": 7, "current_sandboxes": 2})
	q, err := client.Quotas.Get(context.Background(), "u")
	if err != nil || *q.MaxSandboxes != 7 || q.CurrentSandboxes != 2 {
		t.Fatalf("%+v %v", q, err)
	}
}

// ─── Listing with filters ────────────────────────────────────────────────────

func TestSandboxListFiltersTagsAndPaging(t *testing.T) {
	no := false
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data": []map[string]interface{}{{"id": "s1"}, {"id": "s2"}},
		"meta": map[string]interface{}{"page": 2, "limit": 2, "total": 5, "total_pages": 3, "has_next_page": true, "state_counts": map[string]int{"running": 4, "error": 1}, "reserved_memory_mb": 8192},
	})

	page, err := client.Sandboxes.ListPage(context.Background(), miosa.ListSandboxesInput{
		WorkspaceID: "ws_1", ProjectID: "prj_1", ExternalUserID: "dr-smith", State: "running", TemplateID: "miosa-sandbox",
		Search: "build", Sort: "name-asc", Tags: map[string]string{"customer": "acme", "env": "prod"}, IncludeErrored: &no, Page: 2, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/sandboxes")
	want := "external_user_id=dr-smith&include_errored=false&limit=2&page=2&project_id=prj_1&search=build&sort=name-asc&state=running&tags%5Bcustomer%5D=acme&tags%5Benv%5D=prod&template_id=miosa-sandbox&workspace_id=ws_1"
	if got.RawQuery != want {
		t.Errorf("query = %q\nwant    %q", got.RawQuery, want)
	}
	if len(page.Data) != 2 || page.Meta.Total != 5 || !page.Meta.HasNextPage || page.Meta.StateCounts["error"] != 1 || page.Meta.ReservedMemoryMB != 8192 {
		t.Errorf("page = %+v", page)
	}
}

func TestSandboxListAllWalksPages(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "s1"}}, "meta": map[string]interface{}{"has_next_page": true}})
		case "2":
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "s2"}}, "meta": map[string]interface{}{"has_next_page": true}})
		default:
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "s3"}}, "meta": map[string]interface{}{"has_next_page": false}})
		}
	})

	all, err := client.Sandboxes.ListAll(context.Background(), miosa.ListSandboxesInput{Tags: map[string]string{"customer": "acme"}, Page: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[2].ID != "s3" || rec.Count() != 3 {
		t.Errorf("all = %+v after %d requests", all, rec.Count())
	}
}

func TestSandboxListKeepsItsOldBehaviour(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "s1"}}})
	list, err := client.Sandboxes.List(context.Background(), miosa.ListSandboxesInput{ExternalWorkspaceID: "w", ExternalUserID: "u", ExternalProjectID: "p"})
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if got := rec.Last(t).RawQuery; got != "external_project_id=p&external_user_id=u&external_workspace_id=w" {
		t.Errorf("query = %q", got)
	}
}

// ─── Service accounts and the WebSocket dial ─────────────────────────────────

func TestServiceAccounts(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/service-accounts":
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "sa1", "name": "ci", "description": "CI bot", "created_by_user_id": "u1"}}})
		case r.Method == http.MethodDelete:
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sa1", "deleted": true, "revoked_keys": 2}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/keys"):
			writeJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{"id": "k1", "key": "msk_u_secret", "key_prefix": "msk_u_ab", "service_account_id": "sa1", "scopes": []string{"sandboxes:read"}}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/keys"):
			writeJSON(w, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "k1", "service_account_id": "sa1", "status": "active"}}})
		default:
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": "sa1", "name": "ci2"}})
		}
	})
	ctx := context.Background()

	list, err := client.ServiceAccounts.List(ctx)
	if err != nil || list[0].Name != "ci" || list[0].Description != "CI bot" {
		t.Fatalf("%+v %v", list, err)
	}
	if _, err := client.ServiceAccounts.Create(ctx, miosa.CreateServiceAccountInput{Name: "ci2", Description: "d"}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/service-accounts")
	if got.Body(t)["name"] != "ci2" || got.Body(t)["description"] != "d" {
		t.Errorf("body = %v", got.Body(t))
	}
	name := "renamed"
	if _, err := client.ServiceAccounts.Update(ctx, "sa1", miosa.UpdateServiceAccountInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "PATCH", "/service-accounts/sa1")
	del, err := client.ServiceAccounts.Delete(ctx, "sa1")
	if err != nil || !del.Deleted || del.RevokedKeys != 2 {
		t.Fatalf("%+v %v", del, err)
	}
	keys, err := client.ServiceAccounts.Keys(ctx, "sa1")
	if err != nil || keys[0].Status != "active" {
		t.Fatalf("%+v %v", keys, err)
	}
	key, err := client.ServiceAccounts.CreateKey(ctx, "sa1", miosa.ServiceAccountKeyInput{
		Name: "deploy", Scopes: []string{"sandboxes:read"}, AllowedIPs: []string{"10.0.0.0/8"}, WorkspaceID: "ws_1", ExpiresInDays: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = rec.Last(t)
	assertReq(t, got, "POST", "/service-accounts/sa1/keys")
	body := got.Body(t)
	if body["workspace_id"] != "ws_1" || body["expires_in_days"] != float64(30) || body["allowed_ips"].([]interface{})[0] != "10.0.0.0/8" {
		t.Errorf("body = %v", body)
	}
	if key.Key != "msk_u_secret" || key.ServiceAccountID != "sa1" {
		t.Errorf("key = %+v", key)
	}

	if _, err := client.ServiceAccounts.Create(ctx, miosa.CreateServiceAccountInput{}); err == nil {
		t.Error("empty name accepted")
	}
	if _, err := client.ServiceAccounts.CreateKey(ctx, "sa1", miosa.ServiceAccountKeyInput{}); err == nil {
		t.Error("empty scopes accepted")
	}
}

func TestServiceAccountLimitErrors(t *testing.T) {
	client, _ := newFixedClient(t, 429, errorEnvelope("SERVICE_ACCOUNT_LIMIT", "Service account limit reached", nil))
	_, err := client.ServiceAccounts.Create(context.Background(), miosa.CreateServiceAccountInput{Name: "x"})
	var rl *miosa.RateLimitError
	if !errors.As(err, &rl) || rl.Code != "SERVICE_ACCOUNT_LIMIT" {
		t.Fatalf("error = %#v", err)
	}
}

func TestDialWebSocketSendsClientIdentity(t *testing.T) {
	type seen struct {
		hdr   http.Header
		proto string
	}
	got := make(chan seen, 1)
	upgrader := websocket.Upgrader{Subprotocols: []string{"miosa-test-v1"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		got <- seen{hdr: r.Header.Clone(), proto: conn.Subprotocol()}
		conn.Close()
	}))
	defer srv.Close()
	client := miosa.NewClient("msk_u_ws", miosa.WithBaseURL(srv.URL+"/api/v1"), miosa.WithTenant("org-3"), miosa.WithAccessToken("mat_tok"))

	conn, _, err := client.DialWebSocket(miosa.WithBillToContext(context.Background(), "acme"), "/sandboxes/sbx_1/tunnel/3000", "miosa-test-v1")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	s := <-got
	if s.hdr.Get("Authorization") != "Bearer mat_tok" || s.hdr.Get(miosa.HeaderTenant) != "org-3" || s.hdr.Get(miosa.HeaderBillTo) != "acme" {
		t.Errorf("headers = %v", s.hdr)
	}
	if s.proto != "miosa-test-v1" {
		t.Errorf("subprotocol = %q", s.proto)
	}

	if _, _, err := miosa.NewClient("k", miosa.WithBaseURL("http://127.0.0.1:1")).DialWebSocket(context.Background(), "/x"); err == nil {
		t.Error("dialing a dead address must fail")
	}
}
