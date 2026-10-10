package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// These pin the routes the SDK used to call that the API does not serve.

func TestDeploymentCreateAlwaysPostsToDeploymentsWithProjectInBody(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{"id": "dep_1", "project_id": "prj_1"}})

	dep, err := client.Deployments.Create(context.Background(), miosa.CreateDeploymentInput{Name: "web", ProjectID: "prj_1"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/deployments")
	if got.Body(t)["project_id"] != "prj_1" {
		t.Errorf("body = %v", got.Body(t))
	}
	if dep.ID != "dep_1" {
		t.Errorf("deployment = %+v", dep)
	}

	if _, err := client.Deployments.Create(context.Background(), miosa.CreateDeploymentInput{Name: "web"}); err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Last(t).Body(t)["project_id"]; present {
		t.Error("project_id must be omitted when empty")
	}
}

func TestRestoreComputerForksTheSnapshot(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": map[string]interface{}{"id": "cmp_9", "status": "provisioning"}, "snapshot": map[string]interface{}{"id": "snap_1"}})

	c, err := client.RestoreComputer(context.Background(), "snap_1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/snapshots/snap_1/fork")
	if c.ID != "cmp_9" {
		t.Errorf("computer = %+v", c.ComputerData)
	}
}

func TestEgressLockdownTargetsRealPolicyRoutes(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "pol", "mode": "enforce"}})

	if _, err := client.Network.Lockdown(context.Background(), miosa.LockdownInput{}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "PATCH", "/egress/policies/default")

	if _, err := client.Network.Observe(context.Background(), miosa.LockdownInput{ResourceID: "sbx_1", ResourceType: "sandbox"}); err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/egress/policies/sbx_1")
	if got.Body(t)["mode"] != "audit_only" {
		t.Errorf("body = %v", got.Body(t))
	}

	if _, err := client.Network.Lockdown(context.Background(), miosa.LockdownInput{PolicyID: "pol_7"}); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "PATCH", "/egress/policies/pol_7")
}

func TestEgressListsDecodeTheDataArrayTheAPISends(t *testing.T) {
	t.Run("secrets", func(t *testing.T) {
		client, _ := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "s1", "name": "a"}, {"id": "s2"}}})
		list, err := client.Secrets.List(context.Background(), miosa.SecretListInput{})
		if err != nil || len(list) != 2 || list[0].Name != "a" {
			t.Fatalf("%v %v", list, err)
		}
	})
	t.Run("a single secret still decodes", func(t *testing.T) {
		client, _ := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "s1"}})
		list, err := client.Secrets.List(context.Background(), miosa.SecretListInput{})
		if err != nil || len(list) != 1 || list[0].ID != "s1" {
			t.Fatalf("%v %v", list, err)
		}
	})
	t.Run("policies", func(t *testing.T) {
		client, _ := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "p1", "mode": "enforce"}}})
		list, err := client.Network.Policies(context.Background())
		if err != nil || len(list) != 1 || list[0].Mode != "enforce" {
			t.Fatalf("%v %v", list, err)
		}
	})
	t.Run("bindings", func(t *testing.T) {
		client, _ := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{{"id": "b1", "secret_id": "s1"}}})
		list, err := client.Secrets.ListBindings(context.Background(), miosa.BindingListInput{})
		if err != nil || len(list) != 1 || list[0].SecretID != "s1" {
			t.Fatalf("%v %v", list, err)
		}
	})
}

func TestOAuthFlowReportsPendingThenCompleted(t *testing.T) {
	calls := 0
	client, _ := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/egress/oauth/start":
			writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"authorize_url": "https://p/auth", "state": "st"}})
		case "/egress/secrets":
			calls++
			secrets := []map[string]interface{}{{"id": "old", "type": "oauth_connect"}}
			switch {
			case calls >= 3:
				secrets = append(secrets, map[string]interface{}{"id": "other", "type": "api_key"}, map[string]interface{}{"id": "wrong", "type": "oauth_connect", "oauth_provider": "slack"},
					map[string]interface{}{"id": "new", "type": "oauth_connect", "oauth_provider": "github"})
			case calls == 2:
				secrets = append(secrets, map[string]interface{}{"id": "other", "type": "api_key"})
			}
			writeJSON(w, 200, map[string]interface{}{"data": secrets})
		}
	})

	flow, err := client.Secrets.Connect(context.Background(), miosa.OauthConnectInput{Provider: "github"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := flow.Status(context.Background())
	if err != nil || st.Status != "pending" {
		t.Fatalf("first status = %+v, %v", st, err)
	}
	st, err = flow.Status(context.Background())
	if err != nil || st.Status != "completed" || st.SecretID != "new" {
		t.Fatalf("second status = %+v, %v", st, err)
	}
}

func TestScreenshotRegionPostsOnceAndReturnsBytes(t *testing.T) {
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/computers/cmp_1" {
			writeJSON(w, 200, map[string]interface{}{"id": "cmp_1", "status": "running"})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNGdata"))
	})
	c, err := client.Computers.Get(context.Background(), "cmp_1")
	if err != nil {
		t.Fatal(err)
	}

	png, err := c.ScreenshotRegion(context.Background(), 10, 20, 300, 200)
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/computers/cmp_1/desktop/screenshot/region")
	if got.RawQuery != "" {
		t.Errorf("the region rides in the body, got query %q", got.RawQuery)
	}
	body := got.Body(t)
	if body["x"] != float64(10) || body["y"] != float64(20) || body["width"] != float64(300) || body["height"] != float64(200) {
		t.Errorf("body = %v", body)
	}
	if string(png) != "\x89PNGdata" {
		t.Errorf("bytes = %q", png)
	}
	if rec.Count() != 2 {
		t.Errorf("requests = %d; the old GET-then-POST fallback is gone", rec.Count())
	}
}

func TestOpenComputersHostTagsAndUnsupportedUpdate(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"host": map[string]interface{}{"id": "host_1", "tags": []string{"gpu", "eu"}}})

	tags, err := client.OpenComputers.Hosts.SetTags(context.Background(), "host_1", []string{"gpu", "eu"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/opencomputers/hosts/host_1/tags")
	if tags.ID != "host_1" || len(tags.Tags) != 2 || got.Body(t)["tags"].([]interface{})[0] != "gpu" {
		t.Errorf("tags=%+v body=%v", tags, got.Body(t))
	}

	if _, err := client.OpenComputers.Hosts.AddTag(context.Background(), "host_1", "new tag"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "POST", "/opencomputers/hosts/host_1/tags/new tag")

	if _, err := client.OpenComputers.Hosts.RemoveTag(context.Background(), "host_1", "gpu"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/opencomputers/hosts/host_1/tags/gpu")

	before := rec.Count()
	_, err = client.OpenComputers.Hosts.Update(context.Background(), "host_1", miosa.UpdateHostInput{Name: "x"})
	if !miosa.IsCode(err, "UNSUPPORTED") {
		t.Fatalf("Update error = %v", err)
	}
	if rec.Count() != before {
		t.Error("Update must not send a request to a route that does not exist")
	}
}
