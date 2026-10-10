package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestBatchCreateGetItemsCancel(t *testing.T) {
	c, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sandboxes/batch":
			w.WriteHeader(202)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "b1", "status": "pending", "requested_count": 5, "concurrency_limit": 2}})
		case "/sandboxes/batches/b1/items":
			writeJSON(w, 200, map[string]any{"data": []map[string]any{{"id": "s1", "batch_item_index": 0, "state": "running", "ready": true}}})
		default:
			writeJSON(w, 200, map[string]any{"data": map[string]any{"id": "b1", "status": "cancelled", "requested": 5, "settled?": true}})
		}
	})
	ctx := context.Background()
	acc, err := c.Sandboxes.Batches.Create(ctx, miosa.BatchCreateInput{Count: 5, Concurrency: 2, TemplateID: "t1", NamePrefix: "w"})
	if err != nil || acc.ID != "b1" || acc.RequestedCount != 5 || acc.ConcurrencyLimit != 2 {
		t.Fatalf("create = %+v, %v", acc, err)
	}
	assertReq(t, rec.reqs[0], "POST", "/sandboxes/batch")
	if b := rec.reqs[0].Body(t); b["count"].(float64) != 5 || b["template_id"] != "t1" || b["name_prefix"] != "w" {
		t.Fatalf("body = %v", b)
	}
	items, err := c.Sandboxes.Batches.Items(ctx, "b1", 10)
	if err != nil || len(items) != 1 || !items[0].Ready {
		t.Fatalf("items = %+v, %v", items, err)
	}
	if rec.reqs[1].RawQuery != "limit=10" {
		t.Fatalf("query = %q", rec.reqs[1].RawQuery)
	}
	st, err := c.Sandboxes.Batches.Cancel(ctx, "b1")
	if err != nil || !st.Settled || st.Status != "cancelled" {
		t.Fatalf("cancel = %+v, %v", st, err)
	}
	assertReq(t, rec.reqs[2], "POST", "/sandboxes/batches/b1/cancel")
	if _, err := c.Sandboxes.Batches.Create(ctx, miosa.BatchCreateInput{}); err == nil {
		t.Fatal("zero count must fail")
	}
}

func TestBatchWaitPollsUntilSettled(t *testing.T) {
	var n int32
	c, _ := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		settled := atomic.AddInt32(&n, 1) >= 3
		writeJSON(w, 200, map[string]any{"data": map[string]any{"id": "b1", "status": "running", "succeeded": 3, "settled?": settled}})
	})
	st, err := c.Sandboxes.Batches.Wait(context.Background(), "b1", time.Millisecond)
	if err != nil || !st.Settled || n != 3 {
		t.Fatalf("wait = %+v, %v, polls %d", st, err, n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	c2, _ := newFixedClient(t, 200, map[string]any{"data": map[string]any{"id": "b2"}})
	if _, err := c2.Sandboxes.Batches.Wait(ctx, "b2", time.Millisecond); err == nil {
		t.Fatal("expected context error")
	}
}

func TestBatchAlreadyTerminal(t *testing.T) {
	c, _ := newFixedClient(t, 409, errorEnvelope("BATCH_ALREADY_TERMINAL", "done", nil))
	_, err := c.Sandboxes.Batches.Cancel(context.Background(), "b1")
	if !miosa.IsCode(err, "BATCH_ALREADY_TERMINAL") {
		t.Fatalf("err = %v", err)
	}
}

func TestBulkActions(t *testing.T) {
	c, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			writeJSON(w, 200, map[string]any{"job_id": "j1", "action": "pause", "state": "done", "status": "done", "total": 3, "completed": 2, "failed": 1})
			return
		}
		w.WriteHeader(202)
		_ = json.NewEncoder(w).Encode(map[string]any{"job_id": "j1", "queued": 2})
	})
	ctx := context.Background()
	job, err := c.Bulk.SandboxAction(ctx, "pause", []string{"a", "b"})
	if err != nil || job.JobID != "j1" || job.Queued != 2 {
		t.Fatalf("job = %+v, %v", job, err)
	}
	assertReq(t, rec.reqs[0], "POST", "/bulk/sandboxes/pause")
	if _, err := c.Bulk.SandboxActionWhere(ctx, "destroy", miosa.BulkFilter{State: "stopped", WorkspaceID: "w"}); err != nil {
		t.Fatal(err)
	}
	f := rec.reqs[1].Body(t)["filter"].(map[string]any)
	if f["state"] != "stopped" || f["workspace_id"] != "w" {
		t.Fatalf("filter = %v", f)
	}
	if _, err := c.Bulk.ApplyPolicy(ctx, []string{"a"}, map[string]any{"x": 1}, "w"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.reqs[2], "POST", "/bulk/policy/apply")
	if b := rec.reqs[2].Body(t); b["workspace_id"] != "w" || b["policy"] == nil {
		t.Fatalf("body = %v", b)
	}
	got, err := c.Bulk.Job(ctx, "j1")
	if err != nil || got.Total != 3 || got.Completed != 2 || got.Failed != 1 {
		t.Fatalf("status = %+v, %v", got, err)
	}
	if _, err := c.Bulk.SandboxAction(ctx, "", nil); err == nil {
		t.Fatal("empty action must fail")
	}
}

func TestSandboxSpend(t *testing.T) {
	c, rec := newFixedClient(t, 200, map[string]any{"data": map[string]any{
		"period_start": "2026-10-01", "period_end": "2026-11-01", "cap_cents": 5000, "mode": "limited",
		"accounted_cents": 1200, "remaining_cents": 3800, "status": "ok", "alert_thresholds": []int{50, 90},
	}})
	ctx := context.Background()
	s, err := c.SandboxSpend.Get(ctx, "t1")
	if err != nil || s.CapCents == nil || *s.CapCents != 5000 || *s.RemainingCents != 3800 || len(s.AlertThresholds) != 2 {
		t.Fatalf("spend = %+v, %v", s, err)
	}
	assertReq(t, rec.reqs[0], "GET", "/tenants/t1/sandbox-spend")
	cap := int64(7000)
	if _, err := c.SandboxSpend.Update(ctx, "t1", miosa.SandboxSpendUpdate{Mode: "limited", CapCents: &cap}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.reqs[1], "PATCH", "/tenants/t1/sandbox-spend")
	if b := rec.reqs[1].Body(t); b["cap_cents"].(float64) != 7000 || b["mode"] != "limited" {
		t.Fatalf("body = %v", b)
	}
}

func TestComputerStreamEmbedViewer(t *testing.T) {
	c, rec := newFixedClient(t, 200, map[string]any{
		"token": "tok", "expires_at": 1800000000, "ttl_seconds": 600, "computer_id": "c1", "slug": "s",
		"embed_url": "https://e", "desktop_url": "https://d", "desktop_entry_url": "https://de", "ws_url": "wss://w",
		"viewer_password_set": true, "viewer_password": "pw", "viewer_password_set_at": "now",
	})
	ctx := context.Background()
	a, err := c.Computers.StreamToken(ctx, "c1", 600)
	if err != nil || a.Token != "tok" || a.TTLSeconds != 600 {
		t.Fatalf("stream = %+v, %v", a, err)
	}
	assertReq(t, rec.reqs[0], "POST", "/computers/c1/stream-token")
	if rec.reqs[0].Body(t)["ttl_seconds"].(float64) != 600 {
		t.Fatal("ttl not sent")
	}
	if _, err := c.Computers.StreamToken(ctx, "c1", 0); err != nil || len(rec.reqs[1].Raw) != 0 {
		t.Fatalf("default ttl sends no body: %v %q", err, rec.reqs[1].Raw)
	}
	e, err := c.Computers.Embed(ctx, "c1", 120)
	if err != nil || e.EmbedURL != "https://e" || e.WSURL != "wss://w" {
		t.Fatalf("embed = %+v, %v", e, err)
	}
	if rec.reqs[2].RawQuery != "ttl_seconds=120" {
		t.Fatalf("query = %q", rec.reqs[2].RawQuery)
	}
	if _, err := c.Computers.VNCCredentials(ctx, "c1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.reqs[3], "GET", "/computers/c1/vnc-credentials")
	st, err := c.Computers.ViewerPasswordStatus(ctx, "c1")
	if err != nil || !st.ViewerPasswordSet {
		t.Fatalf("status = %+v, %v", st, err)
	}
	rot, err := c.Computers.RotateViewerPassword(ctx, "c1")
	if err != nil || rot.ViewerPassword != "pw" {
		t.Fatalf("rotate = %+v, %v", rot, err)
	}
	assertReq(t, rec.reqs[5], "POST", "/computers/c1/viewer-password/rotate")
	vs, err := c.Computers.CreateViewerSession(ctx, "c1", "secret", 300)
	if err != nil || vs.Token != "tok" {
		t.Fatalf("session = %+v, %v", vs, err)
	}
	if b := rec.reqs[6].Body(t); b["password"] != "secret" || b["ttl_seconds"].(float64) != 300 {
		t.Fatalf("body = %v", b)
	}
	if _, err := c.Computers.CreateViewerSession(ctx, "c1", "", 0); err == nil {
		t.Fatal("empty password must fail")
	}
}

func TestViewerSessionWrongPassword(t *testing.T) {
	c, _ := newFixedClient(t, 401, map[string]any{"error": "wrong_password"})
	_, err := c.Computers.CreateViewerSession(context.Background(), "c1", "x", 0)
	me := miosa.AsMiosaError(err)
	if me == nil || me.StatusCode != 401 {
		t.Fatalf("err = %v", err)
	}
}
