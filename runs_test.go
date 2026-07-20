package miosa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func TestRunsRunWithExecutionPacket(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body miosa.RunCreateInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.SandboxID != "sbx_123" ||
			body.AgentRuntimeProfileID != "arp_123" ||
			body.ExternalWorkspaceID != "cliniciq_ws_123" ||
			body.Instruction != "Use the attached packet and build the requested files." ||
			body.Runner != "claude-code" ||
			body.ExpectedOutputs == nil ||
			len(body.ExpectedOutputs.Files) != 1 ||
			body.ExpectedOutputs.Files[0].Path != "/workspace/output/landing-page.html" ||
			body.ExecutionPacket["goal"] != "Build a lead magnet landing page" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"id":                    "run_123",
				"target_kind":           "sandbox",
				"target_id":             "sbx_123",
				"runner":                body.Runner,
				"instruction":           body.Instruction,
				"status":                "running",
				"external_workspace_id": "cliniciq_ws_123",
			},
		})
	})

	client := newTestClient(t, mux)
	run, err := client.Runs.Run(context.Background(), miosa.RunCreateInput{
		Instruction:           "Use the attached packet and build the requested files.",
		SandboxID:             "sbx_123",
		Runner:                "claude-code",
		Provider:              "anthropic",
		Model:                 "claude-opus-4.8",
		AgentRuntimeProfileID: "arp_123",
		ExternalWorkspaceID:   "cliniciq_ws_123",
		ExecutionPacket: map[string]interface{}{
			"goal": "Build a lead magnet landing page",
		},
		ExpectedOutputs: &miosa.RunExpectedOutputs{
			Files: []miosa.RunExpectedFile{
				{
					Path:     "/workspace/output/landing-page.html",
					Name:     "landing-page.html",
					MimeType: "text/html",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "run_123" || run.Status != miosa.RunStatusRunning {
		t.Fatalf("unexpected run: %#v", run)
	}
}

func TestRunsDownloadFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/runs/run_123/files/file_123/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("disposition") != "inline" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<h1>ClinicIQ</h1>"))
	})

	client := newTestClient(t, mux)
	data, contentType, err := client.Runs.DownloadFile(
		context.Background(),
		"run_123",
		"file_123",
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "text/html" || string(data) != "<h1>ClinicIQ</h1>" {
		t.Fatalf("unexpected file: contentType=%s data=%q", contentType, string(data))
	}
}
