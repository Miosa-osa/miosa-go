package miosa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Miosa-osa/miosa-go"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

// newTestClient creates a Client pointed at a test server.
func newTestClient(t *testing.T, mux *http.ServeMux) *miosa.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return miosa.NewClient("msk_u_test",
		miosa.WithBaseURL(srv.URL),
		miosa.WithMaxRetries(0),
	)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ─── Error handling ───────────────────────────────────────────────────────────

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       interface{}
		wantType   interface{}
	}{
		{
			name:       "401 maps to AuthenticationError",
			statusCode: 401,
			body:       map[string]string{"message": "invalid api key"},
			wantType:   &miosa.AuthenticationError{},
		},
		{
			name:       "402 maps to InsufficientCreditsError",
			statusCode: 402,
			body:       map[string]string{"message": "insufficient credits"},
			wantType:   &miosa.InsufficientCreditsError{},
		},
		{
			name:       "403 maps to PermissionError",
			statusCode: 403,
			body:       map[string]string{"message": "forbidden"},
			wantType:   &miosa.PermissionError{},
		},
		{
			name:       "404 maps to NotFoundError",
			statusCode: 404,
			body:       map[string]string{"message": "not found"},
			wantType:   &miosa.NotFoundError{},
		},
		{
			name:       "422 maps to ValidationError",
			statusCode: 422,
			body:       map[string]string{"message": "invalid input"},
			wantType:   &miosa.ValidationError{},
		},
		{
			name:       "429 maps to RateLimitError",
			statusCode: 429,
			body:       map[string]interface{}{"message": "rate limited", "retry_after": 5.0},
			wantType:   &miosa.RateLimitError{},
		},
		{
			name:       "500 maps to ServerError",
			statusCode: 500,
			body:       map[string]string{"message": "internal error"},
			wantType:   &miosa.ServerError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, tt.statusCode, tt.body)
			})
			client := newTestClient(t, mux)

			_, err := client.Computers.List(context.Background(), miosa.ListComputersInput{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			switch tt.wantType.(type) {
			case *miosa.AuthenticationError:
				var e *miosa.AuthenticationError
				if !errAs(err, &e) {
					t.Fatalf("want AuthenticationError, got %T: %v", err, err)
				}
			case *miosa.InsufficientCreditsError:
				var e *miosa.InsufficientCreditsError
				if !errAs(err, &e) {
					t.Fatalf("want InsufficientCreditsError, got %T: %v", err, err)
				}
			case *miosa.PermissionError:
				var e *miosa.PermissionError
				if !errAs(err, &e) {
					t.Fatalf("want PermissionError, got %T: %v", err, err)
				}
			case *miosa.NotFoundError:
				var e *miosa.NotFoundError
				if !errAs(err, &e) {
					t.Fatalf("want NotFoundError, got %T: %v", err, err)
				}
			case *miosa.ValidationError:
				var e *miosa.ValidationError
				if !errAs(err, &e) {
					t.Fatalf("want ValidationError, got %T: %v", err, err)
				}
			case *miosa.RateLimitError:
				var e *miosa.RateLimitError
				if !errAs(err, &e) {
					t.Fatalf("want RateLimitError, got %T: %v", err, err)
				}
				if e.RetryAfter != 5.0 {
					t.Errorf("RetryAfter: want 5.0, got %v", e.RetryAfter)
				}
			case *miosa.ServerError:
				var e *miosa.ServerError
				if !errAs(err, &e) {
					t.Fatalf("want ServerError, got %T: %v", err, err)
				}
			}
		})
	}
}

// errAs is a minimal errors.As-like check for use in tests without importing "errors".
func errAs[T error](err error, target *T) bool {
	if err == nil {
		return false
	}
	if t, ok := err.(T); ok {
		*target = t
		return true
	}
	return false
}

// ─── Computers service ────────────────────────────────────────────────────────

func TestComputersCreate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.CreateComputerInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, miosa.ComputerData{
			ID:     "cmp_123",
			Name:   input.Name,
			Status: miosa.StatusStopped,
			Size:   input.Size,
		})
	})
	client := newTestClient(t, mux)

	computer, err := client.Computers.Create(context.Background(), miosa.CreateComputerInput{
		Name: "test-computer",
		Size: miosa.SizeSmall,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if computer.ID != "cmp_123" {
		t.Errorf("ID: want cmp_123, got %s", computer.ID)
	}
	if computer.Name != "test-computer" {
		t.Errorf("Name: want test-computer, got %s", computer.Name)
	}
}

func TestComputersList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{
			Data: []miosa.ComputerData{
				{ID: "cmp_1", Name: "alpha", Status: miosa.StatusRunning},
				{ID: "cmp_2", Name: "beta", Status: miosa.StatusStopped},
			},
		})
	})
	client := newTestClient(t, mux)

	result, err := client.Computers.List(context.Background(), miosa.ListComputersInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("want 2 computers, got %d", len(result.Data))
	}
}

func TestComputersGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_abc", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{
			ID:     "cmp_abc",
			Name:   "target",
			Status: miosa.StatusRunning,
		})
	})
	client := newTestClient(t, mux)

	computer, err := client.Computers.Get(context.Background(), "cmp_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if computer.ID != "cmp_abc" {
		t.Errorf("ID: want cmp_abc, got %s", computer.ID)
	}
}

func TestComputersGetNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_missing", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found"})
	})
	client := newTestClient(t, mux)

	_, err := client.Computers.Get(context.Background(), "cmp_missing")
	var nfe *miosa.NotFoundError
	if !errAs(err, &nfe) {
		t.Fatalf("want NotFoundError, got %T: %v", err, err)
	}
}

func TestComputersBash(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers/cmp_xyz/exec", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input miosa.ExecInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		writeJSON(w, http.StatusOK, miosa.ExecResult{
			Output:   fmt.Sprintf("ran: %s", input.Command),
			ExitCode: 0,
			Success:  true,
		})
	})
	// Wire up the computer detail endpoint so Get works.
	mux.HandleFunc("/computers/cmp_xyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, miosa.ComputerData{
			ID:     "cmp_xyz",
			Name:   "exec-test",
			Status: miosa.StatusRunning,
		})
	})
	client := newTestClient(t, mux)

	computer, err := client.Computers.Get(context.Background(), "cmp_xyz")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	result, err := computer.Bash(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("Bash failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode: want 0, got %d", result.ExitCode)
	}
	if result.Output == "" {
		t.Error("Output should not be empty")
	}
}

// ─── Retry behaviour ─────────────────────────────────────────────────────────

func TestRetryOn5xx(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "oops"})
			return
		}
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{
			Data: []miosa.ComputerData{{ID: "cmp_ok", Name: "recovered"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := miosa.NewClient("msk_u_test",
		miosa.WithBaseURL(srv.URL),
		miosa.WithMaxRetries(3),
		miosa.WithTimeout(5*time.Second),
	)

	result, err := client.Computers.List(context.Background(), miosa.ListComputersInput{})
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if len(result.Data) == 0 || result.Data[0].ID != "cmp_ok" {
		t.Errorf("ID: want cmp_ok, got %v", result.Data)
	}
}

// ─── Auth header ─────────────────────────────────────────────────────────────

func TestBearerTokenSent(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{})
	})
	client := newTestClient(t, mux)
	_, _ = client.Computers.List(context.Background(), miosa.ListComputersInput{})

	want := "Bearer msk_u_test"
	if gotAuth != want {
		t.Errorf("Authorization header: want %q, got %q", want, gotAuth)
	}
}

// ─── Context cancellation ────────────────────────────────────────────────────

func TestContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/computers", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		writeJSON(w, http.StatusOK, miosa.ComputerListResponse{})
	})
	client := newTestClient(t, mux)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Computers.List(ctx, miosa.ListComputersInput{})
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
}
