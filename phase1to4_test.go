package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go"
)

func TestTenantPreviewDomain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/preview-domain", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]string{"domain": "preview.acme.com"})
		case http.MethodPut:
			writeJSON(w, 200, map[string]string{"domain": "preview.acme.com"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/tenant/preview-domain/verify", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"verified": true})
	})

	c := newTestClient(t, mux)
	ctx := context.Background()

	got, err := c.Tenant.GetPreviewDomain(ctx)
	if err != nil {
		t.Fatalf("GetPreviewDomain: %v", err)
	}
	if got.Domain != "preview.acme.com" {
		t.Fatalf("expected domain preview.acme.com, got %s", got.Domain)
	}
	if _, err := c.Tenant.SetPreviewDomain(ctx, "preview.acme.com"); err != nil {
		t.Fatalf("SetPreviewDomain: %v", err)
	}
	if _, err := c.Tenant.VerifyPreviewDomain(ctx); err != nil {
		t.Fatalf("VerifyPreviewDomain: %v", err)
	}
}

func TestTenantBranding(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant/branding", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]string{"app_name": "Acme AI"})
		case http.MethodPut:
			writeJSON(w, 200, map[string]string{"app_name": "Acme AI"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	c := newTestClient(t, mux)
	ctx := context.Background()

	got, err := c.Tenant.GetBranding(ctx)
	if err != nil {
		t.Fatalf("GetBranding: %v", err)
	}
	if got.AppName != "Acme AI" {
		t.Fatalf("expected app_name Acme AI, got %s", got.AppName)
	}
	if _, err := c.Tenant.SetBranding(ctx, *got); err != nil {
		t.Fatalf("SetBranding: %v", err)
	}
	if err := c.Tenant.DeleteBranding(ctx); err != nil {
		t.Fatalf("DeleteBranding: %v", err)
	}
}

func TestSandboxFork(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-1/fork", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"id": "sbx-2", "template_type": "miosa-sandbox"})
	})

	c := newTestClient(t, mux)
	got, err := c.Sandboxes.Fork(context.Background(), "sbx-1", miosa.ForkSandboxInput{Name: "fork-of-1"})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if got.ID != "sbx-2" {
		t.Fatalf("expected id sbx-2, got %s", got.ID)
	}
}

func TestSandboxPreviewToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-1/preview-token", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{
			"token":      "mp_tok",
			"url":        "https://example.com?mt=mp_tok",
			"expires_at": "2026-05-27T00:00:00Z",
			"scope":      "read",
		})
	})

	c := newTestClient(t, mux)
	got, err := c.Sandboxes.PreviewToken(context.Background(), "sbx-1", miosa.PreviewTokenInput{})
	if err != nil {
		t.Fatalf("PreviewToken: %v", err)
	}
	if got.Token != "mp_tok" {
		t.Fatalf("expected token mp_tok, got %s", got.Token)
	}
}

func TestQuotas(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quotas/external/usr-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]interface{}{"external_user_id": "usr-1", "max_sandboxes": 5})
		case http.MethodPut:
			writeJSON(w, 200, map[string]interface{}{"external_user_id": "usr-1", "max_sandboxes": 10})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	c := newTestClient(t, mux)
	ctx := context.Background()

	if _, err := c.Quotas.Get(ctx, "usr-1"); err != nil {
		t.Fatalf("Quotas.Get: %v", err)
	}
	maxSB := 10
	if _, err := c.Quotas.Set(ctx, "usr-1", miosa.SetQuotaInput{MaxSandboxes: &maxSB}); err != nil {
		t.Fatalf("Quotas.Set: %v", err)
	}
	if err := c.Quotas.Delete(ctx, "usr-1"); err != nil {
		t.Fatalf("Quotas.Delete: %v", err)
	}
}

func TestUsageRollup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"period_start": "2026-05-01",
			"period_end":   "2026-05-31",
			"results":      []interface{}{},
		})
	})

	c := newTestClient(t, mux)
	got, err := c.Usage.GetRollup(context.Background(), miosa.UsageRollupInput{Period: "30d", GroupBy: "external_user_id"})
	if err != nil {
		t.Fatalf("Usage.GetRollup: %v", err)
	}
	if got.PeriodStart == "" {
		t.Fatal("expected non-empty period_start")
	}
}

func TestSandboxFilesAndShare(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes/sbx-1/files/tree", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"path": "/workspace", "type": "dir", "name": "workspace"})
	})
	mux.HandleFunc("/sandboxes/sbx-1/files/write-many", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"written": []interface{}{}, "failed": []interface{}{}})
	})
	mux.HandleFunc("/sandboxes/sbx-1/shares", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			writeJSON(w, 201, map[string]string{"share_id": "sh-1", "share_url": "https://x.com?ms=tok", "scope": "read"})
		case http.MethodGet:
			writeJSON(w, 200, map[string]interface{}{"data": []interface{}{}})
		}
	})
	mux.HandleFunc("/sandboxes/sbx-1/shares/sh-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClient(t, mux)
	ctx := context.Background()
	h := c.Sandboxes.GetFullHandle("sbx-1")

	tree, err := h.Files.Tree(ctx, "/workspace", 2)
	if err != nil {
		t.Fatalf("Files.Tree: %v", err)
	}
	if tree.Path != "/workspace" {
		t.Fatalf("expected /workspace, got %s", tree.Path)
	}

	if _, err := h.Files.WriteMany(ctx, []miosa.WriteFileInput{
		{Path: "/workspace/hello.txt", ContentBase64: "aGVsbG8="},
	}); err != nil {
		t.Fatalf("Files.WriteMany: %v", err)
	}

	share, err := h.Share.Create(ctx, miosa.CreateShareInput{Scope: "read"})
	if err != nil {
		t.Fatalf("Share.Create: %v", err)
	}
	if share.ShareID != "sh-1" {
		t.Fatalf("expected share_id sh-1, got %s", share.ShareID)
	}
	if _, err := h.Share.List(ctx); err != nil {
		t.Fatalf("Share.List: %v", err)
	}
	if err := h.Share.Revoke(ctx, "sh-1"); err != nil {
		t.Fatalf("Share.Revoke: %v", err)
	}
}
