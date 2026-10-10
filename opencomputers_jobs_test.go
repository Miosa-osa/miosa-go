package miosa_test

import (
	"context"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestOcJobsStart(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusAccepted, map[string]interface{}{
		"job": map[string]interface{}{
			"id": "job_1", "host_id": "host_1", "tenant_id": "t1", "cmd": "make test", "args": []string{"-j4"}, "cwd": "~/app",
			"timeout_ms": 120000, "state": "running", "exit_code": nil, "started_at": "2026-10-10T00:00:00Z",
		},
		"links": map[string]interface{}{
			"job":    "/api/v1/opencomputers/hosts/host_1/exec/job_1",
			"stream": "/api/v1/opencomputers/hosts/host_1/exec/job_1/stream",
		},
	})

	started, err := client.OpenComputers.Jobs.Start(context.Background(), "host_1", miosa.StartJobInput{
		Command: "make test", Args: []string{"-j4"}, Env: map[string]string{"CI": "1"}, Cwd: "~/app", TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/opencomputers/hosts/host_1/jobs")
	body := got.Body(t)
	if body["command"] != "make test" || body["cwd"] != "~/app" || body["timeout"] != float64(120) {
		t.Errorf("body = %v", body)
	}
	if env := body["env"].(map[string]interface{}); env["CI"] != "1" {
		t.Errorf("env = %v", env)
	}
	if started.Job.ID != "job_1" || started.Job.State != "running" || started.Job.TimeoutMS != 120000 || started.Job.Cmd != "make test" || started.Job.ExitCode != nil {
		t.Errorf("job = %+v", started.Job)
	}
	if started.Links.Stream != "/api/v1/opencomputers/hosts/host_1/exec/job_1/stream" {
		t.Errorf("links = %+v", started.Links)
	}
}

func TestOcJobsStartRequiresCommandAndSurfacesHostErrors(t *testing.T) {
	client, rec := newFixedClient(t, http.StatusConflict, map[string]interface{}{"error": "host_not_connected", "job_id": "job_1"})

	if _, err := client.OpenComputers.Jobs.Start(context.Background(), "host_1", miosa.StartJobInput{}); err == nil {
		t.Fatal("empty command accepted")
	}
	if rec.Count() != 0 {
		t.Fatal("reached the server")
	}

	_, err := client.OpenComputers.Jobs.Start(context.Background(), "host_1", miosa.StartJobInput{Command: "ls"})
	m := miosa.AsMiosaError(err)
	if m == nil || m.StatusCode != 409 || m.Message != "host_not_connected" {
		t.Fatalf("error = %#v", err)
	}
}

func TestOcJobsActivity(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"jobs": []map[string]interface{}{
		{"id": "e1", "job_id": "job_1", "kind": "exec", "status": "done", "duration_ms": 812, "dispatched_at": "2026-10-10T00:00:00Z", "completed_at": "2026-10-10T00:00:01Z"},
		{"id": "e2", "job_id": "job_2", "status": "dispatched", "duration_ms": nil},
	}})

	jobs, err := client.OpenComputers.Jobs.Activity(context.Background(), "host_1", 50)
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/opencomputers/hosts/host_1/jobs")
	if got.RawQuery != "limit=50" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if len(jobs) != 2 || jobs[0].Status != "done" || *jobs[0].DurationMS != 812 || jobs[1].DurationMS != nil || jobs[1].Status != "dispatched" {
		t.Errorf("jobs = %+v", jobs)
	}

	if _, err := client.OpenComputers.Jobs.Activity(context.Background(), "host_1", 0); err != nil {
		t.Fatal(err)
	}
	if rec.Last(t).RawQuery != "" {
		t.Errorf("query = %q; the default limit is the server's", rec.Last(t).RawQuery)
	}
}
