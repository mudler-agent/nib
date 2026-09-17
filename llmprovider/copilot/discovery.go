package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// defaultEndpoint is the fallback Copilot API base URL when GraphQL
	// discovery fails or returns nothing.
	defaultEndpoint = "https://api.githubcopilot.com"
	// graphqlURL is the GitHub GraphQL endpoint used for Copilot endpoint
	// discovery.
	graphqlURL = "https://api.github.com/graphql"
	// copilotQuery discovers the Copilot API endpoint.
	copilotQuery = "query { viewer { copilotEndpoints { api } } }"
)

// copilotEndpointsResponse is the GraphQL response shape for the
// copilotEndpoints query.
type copilotEndpointsResponse struct {
	Data struct {
		Viewer struct {
			CopilotEndpoints []struct {
				API string `json:"api"`
			} `json:"copilotEndpoints"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// discoverEndpoint queries GitHub's GraphQL API for the Copilot API base URL.
// Falls back to defaultEndpoint on any error.
func discoverEndpoint(ctx context.Context, client *http.Client, token string) (string, error) {
	body, _ := json.Marshal(map[string]string{"query": copilotQuery})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, strings.NewReader(string(body)))
	if err != nil {
		return defaultEndpoint, nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Editor-Version", editorVersion)

	resp, err := client.Do(req)
	if err != nil {
		return defaultEndpoint, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return defaultEndpoint, nil
	}

	var gqlResp copilotEndpointsResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return defaultEndpoint, nil
	}
	for _, ep := range gqlResp.Data.Viewer.CopilotEndpoints {
		if ep.API != "" {
			return ep.API, nil
		}
	}
	return defaultEndpoint, nil
}

// ResolveToken finds a Copilot OAuth token from environment variables or the
// config files written by gh CLI / Copilot client. It does NOT read the
// credential store — the caller (factory or login command) handles that.
//
// Resolution order:
//  1. GH_COPILOT_TOKEN env var
//  2. COPILOT_GITHUB_TOKEN env var
//  3. ~/.config/github-copilot/hosts.json
//  4. ~/.config/github-copilot/apps.json
//  5. ~/.config/gh/hosts.yml
func ResolveToken() (string, error) {
	for _, env := range []string{"GH_COPILOT_TOKEN", "COPILOT_GITHUB_TOKEN"} {
		if v := os.Getenv(env); v != "" {
			return v, nil
		}
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("copilot: cannot find config dir: %w", err)
	}

	// hosts.json and apps.json: JSON with top-level host keys → {oauth_token}.
	for _, name := range []string{"hosts.json", "apps.json"} {
		if token := readCopilotJSON(filepath.Join(configDir, "github-copilot", name)); token != "" {
			return token, nil
		}
	}

	// gh/hosts.yml: YAML with top-level host keys → {oauth_token: ...}.
	if token := readGhHostsYAML(filepath.Join(configDir, "gh", "hosts.yml")); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("copilot: no token found — install gh CLI and run 'gh auth login', or set GH_COPILOT_TOKEN")
}

// readCopilotJSON reads a github-copilot hosts.json / apps.json file and
// returns the oauth_token for github.com (or the first host found).
func readCopilotJSON(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var hosts map[string]struct {
		OAuthToken string `json:"oauth_token"`
	}
	if err := json.Unmarshal(data, &hosts); err != nil {
		return ""
	}
	// Prefer github.com, fall back to the first entry.
	if h, ok := hosts["github.com"]; ok && h.OAuthToken != "" {
		return h.OAuthToken
	}
	for _, h := range hosts {
		if h.OAuthToken != "" {
			return h.OAuthToken
		}
	}
	return ""
}

// readGhHostsYAML reads a gh CLI hosts.yml file and returns the oauth_token
// for github.com (or the first host found).
func readGhHostsYAML(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var hosts map[string]struct {
		OAuthToken string `yaml:"oauth_token"`
	}
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		return ""
	}
	if h, ok := hosts["github.com"]; ok && h.OAuthToken != "" {
		return h.OAuthToken
	}
	for _, h := range hosts {
		if h.OAuthToken != "" {
			return h.OAuthToken
		}
	}
	return ""
}
