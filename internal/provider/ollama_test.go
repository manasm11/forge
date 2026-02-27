package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================================
// DetectOllama
// ============================================================

func TestDetectOllama_Available(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/api/version" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"version": "0.14.3"})
			return
		}
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "qwen3-coder:latest", "size": 7_600_000_000, "modified_at": "2025-01-15T10:00:00Z",
						"details": map[string]any{"family": "qwen3"}},
					{"name": "glm-4.7-flash:latest", "size": 21_000_000_000, "modified_at": "2025-01-20T12:00:00Z",
						"details": map[string]any{"family": "glm4"}},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	status := DetectOllama(context.Background(), srv.URL)

	if !status.Available {
		t.Fatalf("expected Available=true, got error: %s", status.Error)
	}
	if status.URL != srv.URL {
		t.Errorf("URL = %q", status.URL)
	}
	if len(status.Models) != 2 {
		t.Fatalf("Models count = %d, want 2", len(status.Models))
	}
	if status.Models[0].Name != "qwen3-coder:latest" {
		t.Errorf("Models[0].Name = %q", status.Models[0].Name)
	}
	if status.Models[1].Size != 21_000_000_000 {
		t.Errorf("Models[1].Size = %d", status.Models[1].Size)
	}
	if status.Latency <= 0 {
		t.Error("Latency should be positive")
	}
}

func TestDetectOllama_NotRunning(t *testing.T) {
	t.Parallel()
	// Use a port that nothing is listening on
	status := DetectOllama(context.Background(), "http://127.0.0.1:19999")

	if status.Available {
		t.Error("expected Available=false")
	}
	if status.Error == "" {
		t.Error("Error should be non-empty")
	}
}

func TestDetectOllama_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	status := DetectOllama(context.Background(), srv.URL)

	if status.Available {
		t.Error("expected Available=false on 500")
	}
}

func TestDetectOllama_EmptyURL_UsesDefault(t *testing.T) {
	t.Parallel()
	// Just verify it doesn't panic and returns a status with the default URL
	status := DetectOllama(context.Background(), "")

	// Will likely be unavailable since no real Ollama is running in test
	// but should set the URL to default
	if status.URL != DefaultOllamaURL() {
		t.Errorf("URL should be default, got %q", status.URL)
	}
}

func TestDetectOllama_Timeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	status := DetectOllama(ctx, srv.URL)

	if status.Available {
		t.Error("expected Available=false on timeout")
	}
}

func TestDetectOllama_HealthOK_ModelsEmpty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/api/version" {
			json.NewEncoder(w).Encode(map[string]string{"version": "0.14.3"})
			return
		}
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
	}))
	defer srv.Close()

	status := DetectOllama(context.Background(), srv.URL)

	if !status.Available {
		t.Error("should be available even with no models")
	}
	if len(status.Models) != 0 {
		t.Errorf("Models count = %d, want 0", len(status.Models))
	}
}

func TestDetectOllama_HealthOK_ModelsFail(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/api/version" {
			json.NewEncoder(w).Encode(map[string]string{"version": "0.14.3"})
			return
		}
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}))
	defer srv.Close()

	status := DetectOllama(context.Background(), srv.URL)

	// Server is available but model listing failed
	if !status.Available {
		t.Error("should still be available if health passed")
	}
	if len(status.Models) != 0 {
		t.Errorf("Models should be empty on /api/tags failure")
	}
}

// ============================================================
// ListOllamaModels (standalone)
// ============================================================

func TestListOllamaModels_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "model-a:latest", "size": 1000},
				{"name": "model-b:7b", "size": 2000},
				{"name": "model-c:latest", "size": 3000},
			},
		})
	}))
	defer srv.Close()

	models, err := ListOllamaModels(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("models count = %d, want 3", len(models))
	}
}

func TestListOllamaModels_Error(t *testing.T) {
	t.Parallel()
	models, err := ListOllamaModels(context.Background(), "http://127.0.0.1:19999")
	if err == nil {
		t.Error("expected error")
	}
	if len(models) != 0 {
		t.Errorf("models should be empty on error, got %d", len(models))
	}
}

func TestListOllamaModels_MalformedJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	models, err := ListOllamaModels(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error on malformed JSON")
	}
	if len(models) != 0 {
		t.Errorf("models should be empty, got %d", len(models))
	}
}