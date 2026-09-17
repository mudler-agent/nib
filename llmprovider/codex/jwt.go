package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

const jwtClaimPath = "https://api.openai.com/auth"

// extractAccountID decodes a Codex JWT access token and returns the
// chatgpt_account_id claim, or "" if the token is not a valid Codex JWT.
func extractAccountID(accessToken string) string {
	payload, ok := decodeJWTPayload(accessToken)
	if !ok {
		return ""
	}
	auth, ok := payload[jwtClaimPath].(map[string]any)
	if !ok {
		return ""
	}
	if id, ok := auth["chatgpt_account_id"].(string); ok {
		return id
	}
	return ""
}

// extractResidency decodes a Codex JWT and returns the data residency claim
// (chatgpt_data_residency or chatgpt_compute_residency), or "" if absent.
func extractResidency(accessToken string) string {
	payload, ok := decodeJWTPayload(accessToken)
	if !ok {
		return ""
	}
	auth, ok := payload[jwtClaimPath].(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"chatgpt_data_residency", "chatgpt_compute_residency"} {
		if v, ok := auth[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// decodeJWTPayload base64-decodes the middle segment of a JWT.
func decodeJWTPayload(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard base64 with padding
		decoded, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, false
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, false
	}
	return payload, true
}
