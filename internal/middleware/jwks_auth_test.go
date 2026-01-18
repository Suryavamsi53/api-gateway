package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJWKSMiddleware(t *testing.T) {
	// Create a mock JWKS endpoint
	kid := "test-key-1"
	privateKey := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: big.NewInt(12345),
			E: 65537,
		},
		D: big.NewInt(1),
	}

	// Mock JWKS server
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		jwksData := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": kid,
					"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{0, 1, 0, 1}), // 65537
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksData)
	}))
	defer jwksServer.Close()

	// Create JWKS client and middleware
	jwksClient := NewJWKSClient(jwksServer.URL+"/.well-known/jwks.json", 5*time.Minute)
	mw := NewJWKSMiddleware(jwksClient, "test-issuer", "test-audience")

	// Test valid token
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "user123" {
			t.Fatalf("expected X-User-ID=user123 got=%s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Test missing auth header
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth got %d", rr.Code)
	}
}

func TestJWKSClientCache(t *testing.T) {
	// Create a valid RSA public key
	validN := big.NewInt(12345)

	// Mock JWKS server
	callCount := 0
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		jwksData := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "key1",
					"n":   base64.RawURLEncoding.EncodeToString(validN.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}), // 65537
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksData)
	}))
	defer jwksServer.Close()

	// Create JWKS client with short TTL
	client := NewJWKSClient(jwksServer.URL, 100*time.Millisecond)

	// First call should fetch and cache successfully
	key1, err := client.GetPublicKey("key1")
	if err != nil {
		t.Fatalf("expected valid key on first call, got: %v", err)
	}
	if key1 == nil {
		t.Fatalf("expected non-nil key, got nil")
	}

	// Second call within TTL should use cache (callCount should still be 1)
	key2, _ := client.GetPublicKey("key1")
	if callCount > 1 {
		t.Fatalf("expected 1 fetch within TTL, got %d", callCount)
	}
	if key1.N.Cmp(key2.N) != 0 {
		t.Fatalf("expected same key from cache")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Next call should refresh (callCount becomes 2)
	_, _ = client.GetPublicKey("key1")
	if callCount != 2 {
		t.Fatalf("expected 2 fetches after TTL expiry, got %d", callCount)
	}
}
