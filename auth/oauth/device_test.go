package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mudler/nib/provider"
)

func testDeviceDef(server *httptest.Server) provider.Definition {
	return provider.Definition{
		ID:              "test-device",
		ClientID:        "test-client-id",
		TokenURL:        server.URL + "/token",
		DeviceURL:       server.URL + "/device",
		TokenBodyFormat: "form",
		Scopes:          []string{"openid", "offline_access"},
	}
}

func TestRequestDeviceCodeSuccess(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/device" {
			t.Errorf("path = %s, want /device", r.URL.Path)
		}
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "dc-123",
			"user_code":                 "ABC-XYZ",
			"verification_uri":          "https://example.com/device",
			"verification_uri_complete": "https://example.com/device?user_code=ABC-XYZ",
			"interval":                  1,
			"expires_in":                300,
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr, err := RequestDeviceCode(context.Background(), def)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dr.DeviceCode != "dc-123" {
		t.Errorf("DeviceCode = %q, want dc-123", dr.DeviceCode)
	}
	if dr.UserCode != "ABC-XYZ" {
		t.Errorf("UserCode = %q, want ABC-XYZ", dr.UserCode)
	}
	if dr.VerificationURI != "https://example.com/device" {
		t.Errorf("VerificationURI = %q", dr.VerificationURI)
	}
	if dr.Interval != 1 {
		t.Errorf("Interval = %d, want 1", dr.Interval)
	}
	if dr.ExpiresIn != 300 {
		t.Errorf("ExpiresIn = %d, want 300", dr.ExpiresIn)
	}
	if gotBody == "" {
		t.Error("request body is empty")
	}
	if !strings.Contains(gotBody, "client_id=test-client-id") {
		t.Errorf("body missing client_id: %s", gotBody)
	}
	if !strings.Contains(gotBody, "scope=openid+offline_access") {
		t.Errorf("body missing scope: %s", gotBody)
	}
}

func TestRequestDeviceCodeJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc-json",
			"user_code":   "JSON-1",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	def.TokenBodyFormat = "" // default = json
	dr, err := RequestDeviceCode(context.Background(), def)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dr.DeviceCode != "dc-json" {
		t.Errorf("DeviceCode = %q, want dc-json", dr.DeviceCode)
	}
}

func TestRequestDeviceCodeNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid_client", http.StatusBadRequest)
	}))
	defer server.Close()

	def := testDeviceDef(server)
	_, err := RequestDeviceCode(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestRequestDeviceCodeMissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc-123",
			// missing user_code
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	_, err := RequestDeviceCode(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for missing user_code")
	}
}

func TestRequestDeviceCodeUsesEffectiveClientID(t *testing.T) {
	t.Setenv("TEST_DEVICE_CLIENT_ID", "env-client-id")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if !strings.Contains(body, "client_id=env-client-id") {
			t.Errorf("body missing env client_id: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc-1",
			"user_code":   "UC-1",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	def.EnvClientID = "TEST_DEVICE_CLIENT_ID"
	_, err := RequestDeviceCode(context.Background(), def)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
}

func TestRequestDeviceCodeError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	def := testDeviceDef(server)
	_, err := RequestDeviceCode(context.Background(), def)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500: %v", err)
	}
}

func TestPollOnceSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "tok-123",
			"refresh_token": "ref-456",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	tr, pending, err := pollOnce(context.Background(), def, "dc-123")
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if pending != nil {
		t.Fatal("expected nil pending on success")
	}
	if tr.AccessToken != "tok-123" {
		t.Errorf("AccessToken = %q, want tok-123", tr.AccessToken)
	}
	if tr.RefreshToken != "ref-456" {
		t.Errorf("RefreshToken = %q, want ref-456", tr.RefreshToken)
	}
	if tr.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", tr.ExpiresIn)
	}
}

func TestPollOncePending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "authorization_pending",
			"error_description": "user has not yet authorized",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	_, pending, err := pollOnce(context.Background(), def, "dc-123")
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending error")
	}
	if pending.Error != "authorization_pending" {
		t.Errorf("pending.Error = %q, want authorization_pending", pending.Error)
	}
}

func TestPollOnceUnknownError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "bad code",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	tr, pending, err := pollOnce(context.Background(), def, "dc-bad")
	if err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending error")
	}
	if tr.AccessToken != "" {
		t.Errorf("AccessToken should be empty on error")
	}
	if pending.Error != "invalid_grant" {
		t.Errorf("pending.Error = %q, want invalid_grant", pending.Error)
	}
}

func TestPollDeviceTokenSuccessAfterPending(t *testing.T) {
	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n < 2 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"error":             "authorization_pending",
				"error_description": "waiting",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "tok-success",
			"refresh_token": "ref-success",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr := DeviceResponse{
		DeviceCode: "dc-poll",
		Interval:   1,
		ExpiresIn:  30,
	}
	tr, err := PollDeviceToken(context.Background(), def, dr)
	if err != nil {
		t.Fatalf("PollDeviceToken: %v", err)
	}
	if tr.AccessToken != "tok-success" {
		t.Errorf("AccessToken = %q, want tok-success", tr.AccessToken)
	}
}

func TestPollDeviceTokenAccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "access_denied",
			"error_description": "user denied",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr := DeviceResponse{DeviceCode: "dc-denied", Interval: 1, ExpiresIn: 30}
	_, err := PollDeviceToken(context.Background(), def, dr)
	if err == nil {
		t.Fatal("expected error for access_denied")
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Errorf("error should mention denied: %v", err)
	}
}

func TestPollDeviceTokenExpiredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "expired_token",
			"error_description": "code expired",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr := DeviceResponse{DeviceCode: "dc-exp", Interval: 1, ExpiresIn: 30}
	_, err := PollDeviceToken(context.Background(), def, dr)
	if err == nil {
		t.Fatal("expected error for expired_token")
	}
}

func TestPollDeviceTokenContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "authorization_pending",
			"error_description": "waiting",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr := DeviceResponse{DeviceCode: "dc-cancel", Interval: 10, ExpiresIn: 300}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after the first poll returns pending.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := PollDeviceToken(ctx, def, dr)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestPollDeviceTokenSlowDown(t *testing.T) {
	var pollCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&pollCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"error": "slow_down",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-after-slowdown",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	dr := DeviceResponse{DeviceCode: "dc-slow", Interval: 1, ExpiresIn: 60}
	tr, err := PollDeviceToken(context.Background(), def, dr)
	if err != nil {
		t.Fatalf("PollDeviceToken: %v", err)
	}
	if tr.AccessToken != "tok-after-slowdown" {
		t.Errorf("AccessToken = %q, want tok-after-slowdown", tr.AccessToken)
	}
	if atomic.LoadInt32(&pollCount) < 2 {
		t.Errorf("expected at least 2 polls, got %d", pollCount)
	}
}

func TestPollDeviceTokenExpiredDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "authorization_pending",
		})
	}))
	defer server.Close()

	def := testDeviceDef(server)
	// ExpiresIn=1: deadline is 1 second from now. After the first poll
	// returns pending and we sleep 1s, the deadline check fires.
	dr := DeviceResponse{DeviceCode: "dc-expired", Interval: 1, ExpiresIn: 1}

	_, err := PollDeviceToken(context.Background(), def, dr)
	if err == nil {
		t.Fatal("expected error for expired deadline")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired: %v", err)
	}
}
