package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var presetsJSON = map[string]interface{}{"data": []map[string]interface{}{
	{"name": "read-only", "description": "Every read scope.", "scopes": []string{"sandboxes:read", "deployments:read"}},
	{"name": "ci", "description": "CI pipeline.", "scopes": []string{"sandboxes:read", "sandboxes:create", "sandboxes:exec"}},
}}

func TestApiKeyPresets(t *testing.T) {
	client, rec := newFixedClient(t, 200, presetsJSON)

	presets, err := client.ApiKeys.Presets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/api-keys/presets")
	if len(presets) != 2 || presets[1].Name != "ci" || presets[1].Scopes[2] != "sandboxes:exec" || presets[0].Description == "" {
		t.Errorf("presets = %+v", presets)
	}
}

func TestApiKeyCreateFromPresetUsesServerScopes(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, presetsJSON)
			return
		}
		writeJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{"id": "k1", "name": "ci-key", "key": "msk_u_secret", "key_prefix": "msk_u_ab"}})
	})

	k, err := client.ApiKeys.CreateFromPreset(context.Background(), "ci", miosa.CreateApiKeyInput{Name: "ci-key", ExpiresInDays: 30, WorkspaceID: "ws1"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/api-keys")
	body := got.Body(t)
	scopes := body["scopes"].([]interface{})
	if len(scopes) != 3 || scopes[0] != "sandboxes:read" {
		t.Errorf("scopes = %v", scopes)
	}
	if body["name"] != "ci-key" || body["expires_in_days"] != float64(30) || body["workspace_id"] != "ws1" {
		t.Errorf("body = %v", body)
	}
	if k.Key != "msk_u_secret" || k.KeyPrefix != "msk_u_ab" {
		t.Errorf("key = %+v", k)
	}
}

func TestApiKeyCreateFromUnknownPreset(t *testing.T) {
	client, rec := newFixedClient(t, 200, presetsJSON)

	_, err := client.ApiKeys.CreateFromPreset(context.Background(), "nope", miosa.CreateApiKeyInput{Name: "x"})

	if !miosa.IsCode(err, "UNKNOWN_PRESET") {
		t.Fatalf("error = %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d; only the preset lookup should have been made", rec.Count())
	}
}

func TestApiKeyCreateSendsNewFields(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{
		"id": "k1", "key": "msk_p_x", "key_type": "platform", "key_purpose": "api", "allowed_ips": []string{"10.0.0.0/8"},
		"rate_limit_rpm": 120, "status": "active", "workspace_id": "ws1", "last_four": "wxyz",
	}})

	k, err := client.ApiKeys.Create(context.Background(), miosa.CreateApiKeyInput{
		Name: "svc", KeyType: "platform", AllowedIPs: []string{"10.0.0.0/8"}, RateLimitRPM: 120, ExpiresInDays: 90,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body(t)
	if body["key_type"] != "platform" || body["rate_limit_rpm"] != float64(120) || body["expires_in_days"] != float64(90) {
		t.Errorf("body = %v", body)
	}
	if ips := body["allowed_ips"].([]interface{}); ips[0] != "10.0.0.0/8" {
		t.Errorf("allowed_ips = %v", ips)
	}
	if k.KeyType != "platform" || *k.RateLimitRPM != 120 || k.Status != "active" || k.AllowedIPs[0] != "10.0.0.0/8" || k.LastFour != "wxyz" || k.Key != "msk_p_x" {
		t.Errorf("key = %+v", k)
	}
}

func TestApiKeyCreateBodyUnchangedForExistingCallers(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{"id": "k1"}})
	if _, err := client.ApiKeys.Create(context.Background(), miosa.CreateApiKeyInput{Name: "plain"}); err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body(t)
	if len(body) != 1 || body["name"] != "plain" {
		t.Errorf("an old-style create must send only the name, got %v", body)
	}
}

func TestApiKeyRotate(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{
		"id": "k2", "name": "ci-key", "key": "msk_u_new", "replaces_key_id": "k1", "status": "active",
	}})

	r, err := client.ApiKeys.Rotate(context.Background(), "k1", "ci-key-2")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/api-keys/k1/rotate")
	if got.Body(t)["name"] != "ci-key-2" {
		t.Errorf("body = %v", got.Body(t))
	}
	if r.Key != "msk_u_new" || r.ReplacesKeyID != "k1" || r.ID != "k2" {
		t.Errorf("rotation = %+v", r)
	}

	if _, err := client.ApiKeys.Rotate(context.Background(), "k1", ""); err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Last(t).Body(t)["name"]; present {
		t.Error("an empty name keeps the original's: omit it")
	}
}

func TestApiKeyRotateAlreadyRotated(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("API_KEY_ALREADY_ROTATED", "API key already has a replacement", nil))
	_, err := client.ApiKeys.Rotate(context.Background(), "k1", "")
	if !miosa.IsCode(err, "API_KEY_ALREADY_ROTATED") {
		t.Fatalf("error = %v", err)
	}
}

func TestApiKeyUpdate(t *testing.T) {
	name := "renamed"
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "k1", "name": "renamed", "allowed_ips": []string{"1.2.3.4"}}})

	k, err := client.ApiKeys.Update(context.Background(), "k1", miosa.UpdateApiKeyInput{Name: &name, AllowedIPs: []string{"1.2.3.4"}})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/api-keys/k1")
	if got.Body(t)["name"] != "renamed" || k.Name != "renamed" || k.AllowedIPs[0] != "1.2.3.4" {
		t.Errorf("body=%v key=%+v", got.Body(t), k)
	}
}
