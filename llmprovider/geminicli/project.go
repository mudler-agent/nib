package geminicli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	codeAssistEndpoint = "https://cloudcode-pa.googleapis.com"
	pollInterval      = 5 * time.Second
	pollMaxAttempts   = 24
)

type loadCodeAssistResponse struct {
	CloudAicompanionProject string `json:"cloudaicompanionProject"`
	CurrentTier            *struct {
		ID string `json:"id"`
	} `json:"currentTier"`
	AllowedTiers []struct {
		ID        string `json:"id"`
		IsDefault bool `json:"isDefault"`
	} `json:"allowedTiers"`
}

type longRunningOperation struct {
	Name     string `json:"name"`
	Done     bool   `json:"done"`
	Response *struct {
		CloudAicompanionProject *struct {
			ID string `json:"id"`
		} `json:"cloudaicompanionProject"`
	} `json:"response"`
}

// discoverProject calls the Cloud Code Assist loadCodeAssist and (if needed)
// onboardUser endpoints to discover or provision a project ID for the
// authenticated user. Ported from omp's discoverProject function.
func discoverProject(ctx context.Context, accessToken string) (string, error) {
	envProjectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if envProjectID == "" {
		envProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	}

	headers := http.Header{
		"Authorization":   {"Bearer " + accessToken},
		"Content-Type":    {"application/json"},
		"User-Agent":      {cliUserAgent},
		"Client-Metadata": {clientMetadata},
	}

	data, err := loadCodeAssist(ctx, headers, envProjectID)
	if err != nil {
		return "", err
	}

	if data.CurrentTier != nil {
		if data.CloudAicompanionProject != "" {
			return data.CloudAicompanionProject, nil
		}
		if envProjectID != "" {
			return envProjectID, nil
		}
		return "", fmt.Errorf("gemini-cli: this account requires setting GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_PROJECT_ID")
	}

	tierID := defaultTierID(data.AllowedTiers)
	if tierID != "free-tier" && envProjectID == "" {
		return "", fmt.Errorf("gemini-cli: this account requires setting GOOGLE_CLOUD_PROJECT or GOOGLE_CLOUD_PROJECT_ID")
	}

	op, err := onboardUser(ctx, headers, tierID, envProjectID)
	if err != nil {
		return "", err
	}

	if !op.Done && op.Name != "" {
		op, err = pollOperation(ctx, op.Name, headers)
		if err != nil {
			return "", err
		}
	}

	if op.Response != nil && op.Response.CloudAicompanionProject != nil && op.Response.CloudAicompanionProject.ID != "" {
		return op.Response.CloudAicompanionProject.ID, nil
	}

	if envProjectID != "" {
		return envProjectID, nil
	}

	return "", fmt.Errorf("gemini-cli: could not discover or provision a Google Cloud project — try setting GOOGLE_CLOUD_PROJECT")
}

func loadCodeAssist(ctx context.Context, headers http.Header, envProjectID string) (*loadCodeAssistResponse, error) {
	body := map[string]any{
		"cloudaicompanionProject": envProjectID,
		"metadata": map[string]string{
			"ideType":     "IDE_UNSPECIFIED",
			"platform":    "PLATFORM_UNSPECIFIED",
			"pluginType":  "GEMINI",
			"duetProject": envProjectID,
		},
	}
	respBody, err := postCCA(ctx, codeAssistEndpoint+"/v1internal:loadCodeAssist", headers, body)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli: loadCodeAssist: %w", err)
	}

	var data loadCodeAssistResponse
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("gemini-cli: parse loadCodeAssist: %w", err)
	}
	return &data, nil
}

func onboardUser(ctx context.Context, headers http.Header, tierID, envProjectID string) (*longRunningOperation, error) {
	body := map[string]any{
		"tierId": tierID,
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}
	if tierID != "free-tier" && envProjectID != "" {
		body["cloudaicompanionProject"] = envProjectID
		body["metadata"].(map[string]string)["duetProject"] = envProjectID
	}

	respBody, err := postCCA(ctx, codeAssistEndpoint+"/v1internal:onboardUser", headers, body)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli: onboardUser: %w", err)
	}

	var op longRunningOperation
	if err := json.Unmarshal(respBody, &op); err != nil {
		return nil, fmt.Errorf("gemini-cli: parse onboardUser: %w", err)
	}
	return &op, nil
}

func pollOperation(ctx context.Context, name string, headers http.Header) (*longRunningOperation, error) {
	for attempt := 0; attempt < pollMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(pollInterval):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, codeAssistEndpoint+"/v1internal/"+name, nil)
		if err != nil {
			return nil, fmt.Errorf("gemini-cli: create poll request: %w", err)
		}
		req.Header = headers

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gemini-cli: poll request: %w", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("gemini-cli: poll operation: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}

		var op longRunningOperation
		if err := json.Unmarshal(respBody, &op); err != nil {
			return nil, fmt.Errorf("gemini-cli: parse poll response: %w", err)
		}
		if op.Done {
			return &op, nil
		}
	}

	return nil, fmt.Errorf("gemini-cli: project provisioning did not complete after %d attempts", pollMaxAttempts)
}

func postCCA(ctx context.Context, url string, headers http.Header, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = headers

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func defaultTierID(allowedTiers []struct {
	ID        string `json:"id"`
	IsDefault bool   `json:"isDefault"`
}) string {
	if len(allowedTiers) == 0 {
		return "legacy-tier"
	}
	for _, t := range allowedTiers {
		if t.IsDefault {
			return t.ID
		}
	}
	return "legacy-tier"
}
