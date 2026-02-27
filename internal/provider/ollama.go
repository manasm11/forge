package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ollamaTagsResponse matches Ollama's GET /api/tags response.
type ollamaTagsResponse struct {
	Models []ollamaModelJSON `json:"models"`
}

type ollamaModelJSON struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt string    `json:"modified_at"`
	Details    struct {
		Family string `json:"family"`
	} `json:"details"`
}

// DetectOllama checks if Ollama is running and lists available models.
// If url is empty, DefaultOllamaURL() is used.
// The context controls the overall timeout.
func DetectOllama(ctx context.Context, url string) OllamaStatus {
	if url == "" {
		url = DefaultOllamaURL()
	}

	status := OllamaStatus{URL: url}
	start := time.Now()

	// 1. Health check
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/version", nil)
	if err != nil {
		status.Error = fmt.Sprintf("failed to create request: %v", err)
		return status
	}

	resp, err := client.Do(req)
	status.Latency = time.Since(start)
	if err != nil {
		status.Error = fmt.Sprintf("connection failed: %v", err)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Error = fmt.Sprintf("unhealthy response: HTTP %d", resp.StatusCode)
		return status
	}

	// Parse version if available
	var versionResp map[string]string
	if json.NewDecoder(resp.Body).Decode(&versionResp) == nil {
		status.Version = versionResp["version"]
	}

	status.Available = true

	// 2. List models (best-effort — don't fail the overall detection)
	models, err := ListOllamaModels(ctx, url)
	if err == nil {
		status.Models = models
	}
	// If model listing fails, Available stays true but Models stays empty.

	return status
}

// ListOllamaModels fetches available models from the Ollama API.
func ListOllamaModels(ctx context.Context, url string) ([]OllamaModel, error) {
	if url == "" {
		url = DefaultOllamaURL()
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]OllamaModel, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		var modTime time.Time
		if m.ModifiedAt != "" {
			modTime, _ = time.Parse(time.RFC3339, m.ModifiedAt)
		}
		models = append(models, OllamaModel{
			Name:       m.Name,
			Size:       m.Size,
			Family:     m.Details.Family,
			ModifiedAt: modTime,
		})
	}

	return models, nil
}