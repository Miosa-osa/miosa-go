package miosa_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Miosa-osa/miosa-go"
)

func TestFilesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		writeJSON(w, http.StatusOK, miosa.FileListResult{
			Path: path,
			Entries: []miosa.FileEntry{
				{Name: "hello.txt", Path: path + "/hello.txt", Size: 128, IsDir: false},
				{Name: "subdir", Path: path + "/subdir", Size: 0, IsDir: true},
			},
		})
	})
	client := newTestClient(t, mux)

	files := client.Files.For("cmp_f")
	result, err := files.List(context.Background(), "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Errorf("want 2 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Name != "hello.txt" {
		t.Errorf("first entry name: want hello.txt, got %s", result.Entries[0].Name)
	}
	if !result.Entries[1].IsDir {
		t.Error("second entry should be a directory")
	}
}

func TestFilesDownload(t *testing.T) {
	content := "hello from the computer"
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, content)
	})
	client := newTestClient(t, mux)

	data, err := client.Files.For("cmp_f").Download(context.Background(), "/home/user/hello.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != content {
		t.Errorf("content: want %q, got %q", content, string(data))
	}
}

func TestFilesDownloadTo(t *testing.T) {
	content := "streamed content"
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files/download", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, content)
	})
	client := newTestClient(t, mux)

	var buf strings.Builder
	err := client.Files.For("cmp_f").DownloadTo(context.Background(), "/path/file.txt", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != content {
		t.Errorf("content: want %q, got %q", content, buf.String())
	}
}

func TestFilesUploadReader(t *testing.T) {
	var gotPath string
	var gotFilename string
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotPath = r.FormValue("path")
		file, header, err := r.FormFile("file")
		if err == nil {
			gotFilename = header.Filename
			file.Close()
		}
		w.WriteHeader(http.StatusCreated)
	})
	client := newTestClient(t, mux)

	content := strings.NewReader("file contents")
	err := client.Files.For("cmp_f").UploadReader(context.Background(), content, "local.txt", "/remote/local.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/remote/local.txt" {
		t.Errorf("path: want /remote/local.txt, got %s", gotPath)
	}
	if gotFilename != "local.txt" {
		t.Errorf("filename: want local.txt, got %s", gotFilename)
	}
}

func TestFilesExport(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files/export", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, http.StatusOK, miosa.FileExportResult{
			URL:       "https://cdn.example.com/file.txt?token=abc",
			ExpiresAt: "2026-01-01T00:00:00Z",
		})
	})
	client := newTestClient(t, mux)

	result, err := client.Files.For("cmp_f").Export(context.Background(), "/home/user/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestFilesDelete(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_f/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	client := newTestClient(t, mux)

	err := client.Files.For("cmp_f").Delete(context.Background(), "/home/user/old.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("DELETE was not called")
	}
}
