package miosa_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var snapJSON = map[string]interface{}{
	"id":       "f3c1",
	"resource": map[string]interface{}{"type": "sandbox", "id": "9a2e", "name": "builder", "state": "paused", "workspace_id": "1b7d"},
	"status":   "ready", "comment": nil, "size_bytes": 412345678, "disk_bytes": 4294967296, "memory_bytes": 536870912,
	"keep": false, "named_snapshot": nil, "browsable": true, "parent_snapshot_id": nil, "error": nil,
	"created_at": "2026-10-09T18:00:00Z", "expires_at": "2026-11-08T18:00:00Z", "last_used_at": "2026-10-09T18:00:00Z",
}

func TestAccountSnapshotsList(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []interface{}{snapJSON}})

	snaps, err := client.Snapshots.List(context.Background(), miosa.ListSnapshotsOptions{
		ResourceID: "9a2e", WorkspaceID: "1b7d", Query: "build_er", Limit: 25, Offset: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/snapshots")
	if got.RawQuery != "limit=25&offset=50&q=build_er&resource_id=9a2e&workspace_id=1b7d" {
		t.Errorf("query = %q", got.RawQuery)
	}
	s := snaps[0]
	if s.Resource.Type != "sandbox" || s.Resource.Name != "builder" || s.Status != "ready" || !s.Browsable || s.DiskBytes != 4294967296 {
		t.Errorf("snapshot = %+v", s)
	}
	if s.NamedSnapshot != nil || s.Keep {
		t.Errorf("history snapshots are unpinned: %+v", s)
	}
}

func TestAccountSnapshotsGroupsDropsResourceFilter(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": []map[string]interface{}{
		{"resource_id": "9a2e", "resource": nil, "snapshot_count": 12, "stored_bytes": 4123456789, "latest_at": "2026-10-09T18:00:00Z"},
	}})

	groups, err := client.Snapshots.Groups(context.Background(), miosa.ListSnapshotsOptions{ResourceID: "ignored", WorkspaceID: "1b7d"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/snapshots/groups")
	if got.RawQuery != "workspace_id=1b7d" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if groups[0].SnapshotCount != 12 || groups[0].Resource != nil || groups[0].StoredBytes != 4123456789 {
		t.Errorf("group = %+v", groups[0])
	}
}

func TestAccountSnapshotsGetWithDependents(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data":       snapJSON,
		"dependents": []map[string]interface{}{{"type": "named_snapshot", "id": "n1", "label": "web-stack"}},
	})

	d, err := client.Snapshots.Get(context.Background(), "f3c1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/snapshots/f3c1")
	if d.Snapshot.ID != "f3c1" || len(d.Dependents) != 1 || d.Dependents[0].Label != "web-stack" {
		t.Errorf("detail = %+v", d)
	}
}

func TestAccountSnapshotsDelete(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"id": "f3c1", "deleted": true}})
	if err := client.Snapshots.Delete(context.Background(), "f3c1"); err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "DELETE", "/snapshots/f3c1")
}

func TestAccountSnapshotsDeleteManyPartial(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"deleted":   []string{"f3c1"},
		"blocked":   []map[string]interface{}{{"snapshot_id": "a77e", "dependents": []map[string]interface{}{{"type": "child_snapshot", "id": "c"}}}},
		"not_found": []string{},
	}})

	res, err := client.Snapshots.DeleteMany(context.Background(), miosa.DeleteSnapshotsInput{IDs: []string{"f3c1", "a77e"}})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/snapshots/delete")
	if ids := got.Body(t)["ids"].([]interface{}); len(ids) != 2 {
		t.Errorf("ids = %v", ids)
	}
	if len(res.Deleted) != 1 || len(res.Blocked) != 1 || res.Blocked[0].Dependents[0].Type != "child_snapshot" {
		t.Errorf("result = %+v", res)
	}
}

func TestAccountSnapshotsDeleteManyAllBlockedIs409ButStillReturnsResult(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusConflict, map[string]interface{}{"data": map[string]interface{}{
		"deleted": []string{},
		"blocked": []map[string]interface{}{{"snapshot_id": "a77e", "dependents": []map[string]interface{}{{"type": "named_snapshot"}}}},
	}})

	res, err := client.Snapshots.DeleteMany(context.Background(), miosa.DeleteSnapshotsInput{IDs: []string{"a77e"}})

	if err != nil {
		t.Fatalf("a fully blocked bulk delete reports through the result: %v", err)
	}
	if len(res.Blocked) != 1 || res.Blocked[0].SnapshotID != "a77e" {
		t.Errorf("result = %+v", res)
	}
}

func TestAccountSnapshotsDeleteManyWholeHistoryNeedsConfirm(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{"deleted": []string{"a"}}})

	_, err := client.Snapshots.DeleteMany(context.Background(), miosa.DeleteSnapshotsInput{ResourceID: "9a2e", All: true, Confirm: "9a2e"})
	if err != nil {
		t.Fatal(err)
	}
	body := rec.Last(t).Body(t)
	if body["resource_id"] != "9a2e" || body["all"] != true || body["confirm"] != "9a2e" {
		t.Errorf("body = %v", body)
	}
	if _, present := body["ids"]; present {
		t.Error("ids must be omitted for a history delete")
	}
}

func TestAccountSnapshotsDeleteManyOtherErrorsPropagate(t *testing.T) {
	client, _ := newFixedClient(t, 422, errorEnvelope("INVALID_REQUEST", "confirm must equal resource_id", nil))
	_, err := client.Snapshots.DeleteMany(context.Background(), miosa.DeleteSnapshotsInput{ResourceID: "x", All: true})
	if !miosa.IsCode(err, "INVALID_REQUEST") {
		t.Fatalf("error = %v", err)
	}
}

func TestAccountSnapshotsFork(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{
		"data":     map[string]interface{}{"machine_type": "sandbox", "id": "c4d0", "state": "provisioning"},
		"snapshot": snapJSON,
	})

	res, err := client.Snapshots.Fork(context.Background(), "f3c1", miosa.ForkSnapshotInput{
		MachineEnvironmentOptions: miosa.MachineEnvironmentOptions{Environment: "staging", NoEnv: false},
		IdempotencyKey:            "fork-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/snapshots/f3c1/fork")
	if got.Header.Get("Idempotency-Key") != "fork-1" || got.Body(t)["environment"] != "staging" {
		t.Errorf("header=%v body=%v", got.Header, got.Body(t))
	}
	if res.MachineType != "sandbox" || res.ID != "c4d0" || res.State != "provisioning" || res.Snapshot == nil || res.Snapshot.ID != "f3c1" {
		t.Errorf("result = %+v", res)
	}
	if len(res.Machine) == 0 {
		t.Error("raw machine JSON should be kept")
	}
}

func TestAccountSnapshotsTree(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
		"path": "/home/user",
		"entries": []map[string]interface{}{
			{"name": "src", "path": "/home/user/src", "type": "directory", "size": 4096, "mode": "0755"},
			{"name": "current", "path": "/home/user/current", "type": "symlink", "size": 9, "mode": "0777"},
		},
	}})

	tree, err := client.Snapshots.Tree(context.Background(), "f3c1", "/home/user", miosa.BrowseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/snapshots/f3c1/tree")
	if got.RawQuery != "path=%2Fhome%2Fuser" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if tree.Path != "/home/user" || len(tree.Entries) != 2 || tree.Entries[0].Type != "directory" || tree.Entries[1].Type != "symlink" {
		t.Errorf("tree = %+v", tree)
	}
}

func TestAccountSnapshotsTreePollsWhileWarming(t *testing.T) {
	var calls int32
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"status": "warming", "retry_after_ms": 5})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"path": "/", "entries": []interface{}{}}})
	})

	tree, err := client.Snapshots.Tree(context.Background(), "f3c1", "", miosa.BrowseOptions{WaitTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Count() != 3 || tree.Path != "/" {
		t.Errorf("requests = %d, tree = %+v", rec.Count(), tree)
	}
	if rec.Last(t).RawQuery != "" {
		t.Errorf("an empty path must be omitted, got %q", rec.Last(t).RawQuery)
	}
}

func TestAccountSnapshotsTreeWaitDisabledReturnsWarmingError(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusAccepted, map[string]interface{}{"status": "warming", "retry_after_ms": 2000})

	_, err := client.Snapshots.Tree(context.Background(), "f3c1", "/", miosa.BrowseOptions{WaitTimeout: -1})

	var warming *miosa.SnapshotWarmingError
	if !errors.As(err, &warming) || warming.RetryAfter != 2*time.Second {
		t.Fatalf("error = %#v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("requests = %d", rec.Count())
	}
}

func TestAccountSnapshotsTreeWarmingTimesOut(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusAccepted, map[string]interface{}{"status": "warming", "retry_after_ms": 500})

	_, err := client.Snapshots.Tree(context.Background(), "f3c1", "/", miosa.BrowseOptions{WaitTimeout: 50 * time.Millisecond})

	var warming *miosa.SnapshotWarmingError
	if !errors.As(err, &warming) {
		t.Fatalf("error = %#v", err)
	}
}

func TestAccountSnapshotsDownloadStreamsAndPollsWarming(t *testing.T) {
	var calls int32
	client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"status": "warming", "retry_after_ms": 5})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("file-bytes"))
	})

	var buf bytes.Buffer
	n, err := client.Snapshots.Download(context.Background(), "f3c1", "/home/user/notes.txt", &buf, miosa.BrowseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 10 || buf.String() != "file-bytes" {
		t.Errorf("n=%d body=%q", n, buf.String())
	}
	assertReq(t, rec.Last(t), "GET", "/snapshots/f3c1/files")

	data, err := client.Snapshots.DownloadBytes(context.Background(), "f3c1", "/x", miosa.BrowseOptions{})
	if err != nil || string(data) != "file-bytes" {
		t.Errorf("DownloadBytes = %q, %v", data, err)
	}
}

func TestAccountSnapshotsDownloadErrors(t *testing.T) {
	client, _ := newFixedClient(t, 413, errorEnvelope("TOO_LARGE", "folder is over 1 GiB", nil))
	_, err := client.Snapshots.DownloadBytes(context.Background(), "f3c1", "/big", miosa.BrowseOptions{})
	if !miosa.IsCode(err, "TOO_LARGE") {
		t.Fatalf("error = %v", err)
	}
}

// ─── Named snapshots ──────────────────────────────────────────────────────────

var namedJSON = map[string]interface{}{
	"id": "n1", "name": "web-stack", "status": "ready", "snapshot_id": "f3c1", "workspace_id": "1b7d",
	"source":   map[string]interface{}{"type": "sandbox", "id": "9a2e", "name": "builder"},
	"promoted": true, "size_bytes": 412345678, "disk_bytes": 4294967296, "browsable": true, "error": nil,
}

func TestNamedSnapshotsList(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{
		"data": []interface{}{namedJSON},
		"allowance": map[string]interface{}{"included": 10, "used": 3, "remaining_free": 7, "billable": 0,
			"monthly_price_cents": 170, "monthly_cost_cents": 0},
	})

	list, err := client.NamedSnapshots.List(context.Background(), "1b7d")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/named-snapshots")
	if got.RawQuery != "workspace_id=1b7d" {
		t.Errorf("query = %q", got.RawQuery)
	}
	n := list.Data[0]
	if n.Name != "web-stack" || n.Status != "ready" || !n.Promoted || n.Source.Type != "sandbox" || n.SnapshotID != "f3c1" {
		t.Errorf("named = %+v", n)
	}
	if list.Allowance.Included != 10 || list.Allowance.RemainingFree != 7 || list.Allowance.MonthlyPriceCents != 170 {
		t.Errorf("allowance = %+v", list.Allowance)
	}
}

func TestNamedSnapshotsGet(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": namedJSON, "dependents": []map[string]interface{}{{"type": "suspended_computer", "id": "c9"}}})

	d, err := client.NamedSnapshots.Get(context.Background(), "web-stack")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/named-snapshots/web-stack")
	if d.Snapshot.Name != "web-stack" || d.Dependents[0].Type != "suspended_computer" {
		t.Errorf("detail = %+v", d)
	}
}

func TestNamedSnapshotsSave(t *testing.T) {
	saving := map[string]interface{}{}
	for k, v := range namedJSON {
		saving[k] = v
	}
	saving["status"] = "saving"
	client, rec := newFixedClient(t, 201, map[string]interface{}{"data": saving, "operation_id": "op_1"})

	saved, err := client.NamedSnapshots.Save(context.Background(), miosa.SaveNamedSnapshotInput{Name: "web-stack", SandboxID: "9a2e"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/named-snapshots")
	body := got.Body(t)
	if body["name"] != "web-stack" || body["sandbox_id"] != "9a2e" {
		t.Errorf("body = %v", body)
	}
	if _, present := body["snapshot_id"]; present {
		t.Error("exactly one source: snapshot_id must be omitted")
	}
	if saved.Status != "saving" || saved.OperationID != "op_1" || saved.Name != "web-stack" {
		t.Errorf("saved = %+v", saved)
	}
}

func TestNamedSnapshotsSaveErrors(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("NAME_TAKEN", "name exists", nil))
	if _, err := client.NamedSnapshots.Save(context.Background(), miosa.SaveNamedSnapshotInput{Name: "x", SnapshotID: "s"}); !miosa.IsCode(err, "NAME_TAKEN") {
		t.Errorf("error = %v", err)
	}
	client, _ = newFixedClient(t, 402, errorEnvelope("CREDIT_REQUIRED", "past the free allowance", nil))
	_, err := client.NamedSnapshots.Save(context.Background(), miosa.SaveNamedSnapshotInput{Name: "x", SnapshotID: "s"})
	var credits *miosa.InsufficientCreditsError
	if !errors.As(err, &credits) || credits.Code != miosa.CodeCreditRequired {
		t.Errorf("error = %#v", err)
	}
	if _, err := client.NamedSnapshots.Save(context.Background(), miosa.SaveNamedSnapshotInput{}); err == nil {
		t.Error("empty name accepted")
	}
}

func TestNamedSnapshotsRemoveOutcome(t *testing.T) {
	for _, outcome := range []miosa.NamedSnapshotOutcome{miosa.NamedSnapshotReleased, miosa.NamedSnapshotDeleted, miosa.NamedSnapshotKept} {
		t.Run(string(outcome), func(t *testing.T) {
			client, rec := newFixedClient(t, 200, map[string]interface{}{"data": map[string]interface{}{
				"name": "web-stack", "removed": true, "snapshot_id": "f3c1", "outcome": string(outcome),
			}})
			res, err := client.NamedSnapshots.Remove(context.Background(), "web-stack")
			if err != nil {
				t.Fatal(err)
			}
			assertReq(t, rec.Last(t), "DELETE", "/named-snapshots/web-stack")
			if res.Outcome != outcome || !res.Removed || res.SnapshotID != "f3c1" {
				t.Errorf("result = %+v", res)
			}
		})
	}
}

func TestNamedSnapshotsDeploy(t *testing.T) {
	client, rec := newFixedClient(t, 201, map[string]interface{}{
		"data":           map[string]interface{}{"machine_type": "computer", "id": "cmp_9", "state": "provisioning"},
		"named_snapshot": "web-stack",
	})

	res, err := client.NamedSnapshots.Deploy(context.Background(), "web-stack", miosa.ForkSnapshotInput{
		MachineEnvironmentOptions: miosa.MachineEnvironmentOptions{EnvironmentID: "env_1"}, IdempotencyKey: "d-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/named-snapshots/web-stack/deploy")
	if got.Header.Get("Idempotency-Key") != "d-1" || got.Body(t)["environment_id"] != "env_1" {
		t.Errorf("header=%v body=%v", got.Header, got.Body(t))
	}
	if res.MachineType != "computer" || res.NamedSnapshot != "web-stack" || res.ID != "cmp_9" {
		t.Errorf("result = %+v", res)
	}
}

func TestNamedSnapshotsDeployNotReadyIsRetryable(t *testing.T) {
	client, _ := newFixedClient(t, 409, errorEnvelope("SNAPSHOT_NOT_READY", "still saving", map[string]interface{}{"retryable": true}))

	_, err := client.NamedSnapshots.Deploy(context.Background(), "web-stack", miosa.ForkSnapshotInput{})

	m := miosa.AsMiosaError(err)
	if m == nil || !m.Retryable || m.Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("error = %#v", err)
	}
}

func TestNamedSnapshotNamesArePathEscaped(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"data": namedJSON})
	if _, err := client.NamedSnapshots.Get(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.Path != "/named-snapshots/a/b" {
		t.Errorf("path = %q", got.Path)
	}
}
