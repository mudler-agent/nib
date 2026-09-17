package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mudler/nib/provider"
)

// DeviceGrantType is the RFC 8628 grant type for device-code token requests.
const DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// DeviceResponse is the result of a device authorization request (RFC 8628 §3.1).
type DeviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

// deviceErrorResponse is the error JSON returned by the token endpoint while
// the user has not yet authorized the device.
type deviceErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// RequestDeviceCode starts a device-code flow by POSTing to the provider's
// device authorization endpoint. The returned DeviceResponse contains the
// user code and verification URI to display to the user.
func RequestDeviceCode(ctx context.Context, def provider.Definition) (DeviceResponse, error) {
	body := map[string]string{
		"client_id": def.EffectiveClientID(),
	}
	if len(def.Scopes) > 0 {
		body["scope"] = strings.Join(def.Scopes, " ")
	}

	req, err := newRequest(ctx, http.MethodPost, def.DeviceURL, body, def)
	if err != nil {
		return DeviceResponse{}, fmt.Errorf("oauth: create device-code request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DeviceResponse{}, fmt.Errorf("oauth: device-code request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return DeviceResponse{}, fmt.Errorf("oauth: read device-code response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return DeviceResponse{}, fmt.Errorf("oauth: device-code endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var dr DeviceResponse
	if err := json.Unmarshal(respBody, &dr); err != nil {
		return DeviceResponse{}, fmt.Errorf("oauth: parse device-code response: %w", err)
	}
	if dr.DeviceCode == "" || dr.UserCode == "" {
		return DeviceResponse{}, fmt.Errorf("oauth: device-code response missing required fields")
	}
	return dr, nil
}

// PollDeviceToken polls the token endpoint until the user authorizes the device,
// the device code expires, or ctx is cancelled. It returns the token response
// on success or an error on failure.
func PollDeviceToken(ctx context.Context, def provider.Definition, dr DeviceResponse) (TokenResponse, error) {
	interval := dr.Interval
	if interval < 1 {
		interval = 5
	}

	var deadline time.Time
	if dr.ExpiresIn > 0 {
		deadline = time.Now().Add(time.Duration(dr.ExpiresIn) * time.Second)
	}

	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return TokenResponse{}, fmt.Errorf("oauth: device code expired")
		}

		select {
		case <-ctx.Done():
			return TokenResponse{}, ctx.Err()
		default:
		}

		tr, pending, err := pollOnce(ctx, def, dr.DeviceCode)
		if err != nil {
			return TokenResponse{}, err
		}
		if pending == nil {
			return tr, nil
		}

		switch pending.Error {
		case "authorization_pending":
			if !sleep(ctx, time.Duration(interval)*time.Second) {
				return TokenResponse{}, ctx.Err()
			}
		case "slow_down":
			interval += 5
			if !sleep(ctx, time.Duration(interval)*time.Second) {
				return TokenResponse{}, ctx.Err()
			}
		case "expired_token":
			return TokenResponse{}, fmt.Errorf("oauth: device code expired")
		case "access_denied":
			return TokenResponse{}, fmt.Errorf("oauth: authorization denied by user")
		default:
			return TokenResponse{}, fmt.Errorf("oauth: device flow error: %s: %s", pending.Error, pending.ErrorDescription)
		}
	}
}

// pollOnce sends one token request and returns either a successful
// TokenResponse, a pending error, or a hard error.
func pollOnce(ctx context.Context, def provider.Definition, deviceCode string) (TokenResponse, *deviceErrorResponse, error) {
	body := map[string]string{
		"grant_type":  DeviceGrantType,
		"device_code": deviceCode,
		"client_id":   def.EffectiveClientID(),
	}

	req, err := newRequest(ctx, http.MethodPost, def.TokenURL, body, def)
	if err != nil {
		return TokenResponse{}, nil, fmt.Errorf("oauth: create device poll request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenResponse{}, nil, fmt.Errorf("oauth: device poll request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, nil, fmt.Errorf("oauth: read device poll response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var tr TokenResponse
		if err := json.Unmarshal(respBody, &tr); err != nil {
			return TokenResponse{}, nil, fmt.Errorf("oauth: parse device token response: %w", err)
		}
		return tr, nil, nil
	}

	// Non-200: parse the standard OAuth error response.
	var derr deviceErrorResponse
	if err := json.Unmarshal(respBody, &derr); err != nil {
		return TokenResponse{}, nil, fmt.Errorf("oauth: device poll returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return TokenResponse{}, &derr, nil
}

// sleep waits for d or until ctx is cancelled. Returns false if ctx was cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
