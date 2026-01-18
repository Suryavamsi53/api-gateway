package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponseCache_GetSet(t *testing.T) {
	rc := NewResponseCache(100, 1024*1024)

	entry := &CacheEntry{
		Status:    200,
		Body:      []byte("test"),
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	rc.Set("test-key", entry)
	retrieved, exists := rc.Get("test-key")

	if !exists {
		t.Error("expected entry to exist")
	}

	if retrieved.HitCount != 1 {
		t.Errorf("expected hit count 1, got %d", retrieved.HitCount)
	}

	if string(retrieved.Body) != "test" {
		t.Errorf("expected body 'test', got %s", string(retrieved.Body))
	}
}

func TestResponseCache_Expiration(t *testing.T) {
	rc := NewResponseCache(100, 1024*1024)

	entry := &CacheEntry{
		Status:    200,
		Body:      []byte("test"),
		ExpiresAt: time.Now().Add(-1 * time.Second), // Expired
	}

	rc.Set("test-key", entry)
	_, exists := rc.Get("test-key")

	if exists {
		t.Error("expected entry to be expired")
	}
}

func TestResponseCache_SizeLimit(t *testing.T) {
	rc := NewResponseCache(2, 1024*1024) // Max 2 entries

	for i := 0; i < 3; i++ {
		entry := &CacheEntry{
			Status:    200,
			Body:      []byte("test"),
			ExpiresAt: time.Now().Add(1 * time.Minute),
		}
		rc.Set("key"+string(rune(i)), entry)
	}

	if rc.GetSize() > 2 {
		t.Errorf("expected size <= 2, got %d", rc.GetSize())
	}
}

func TestResponseCache_Clear(t *testing.T) {
	rc := NewResponseCache(100, 1024*1024)

	entry := &CacheEntry{
		Status:    200,
		Body:      []byte("test"),
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}
	rc.Set("key1", entry)
	rc.Set("key2", entry)

	rc.Clear()

	if rc.GetSize() != 0 {
		t.Errorf("expected size 0 after clear, got %d", rc.GetSize())
	}
}

func TestCachedRoundTripper_CachesGET(t *testing.T) {
	rc := NewResponseCache(100, 1024*1024)
	crt := NewCachedRoundTripper(rc)

	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Cache-Control", "max-age=300")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	// Create HTTP client with cached transport
	client := &http.Client{Transport: crt}

	// First request - should hit server
	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp1, _ := client.Do(req)
	resp1.Body.Close()

	if callCount != 1 {
		t.Errorf("expected 1 server call, got %d", callCount)
	}

	// Second request - should use cache
	req, _ = http.NewRequest("GET", server.URL+"/test", nil)
	resp2, _ := client.Do(req)
	resp2.Body.Close()

	if callCount != 1 {
		t.Errorf("expected 1 server call (cached), got %d", callCount)
	}

	if rc.GetSize() != 1 {
		t.Errorf("expected 1 cache entry, got %d", rc.GetSize())
	}
}

func TestCachedRoundTripper_SkipsPOST(t *testing.T) {
	rc := NewResponseCache(100, 1024*1024)
	crt := NewCachedRoundTripper(rc)

	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer server.Close()

	client := &http.Client{Transport: crt}

	// POST request - should not be cached
	req, _ := http.NewRequest("POST", server.URL+"/test", nil)
	resp, _ := client.Do(req)
	resp.Body.Close()

	if rc.GetSize() != 0 {
		t.Errorf("expected no cache for POST, got %d entries", rc.GetSize())
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey("GET", "/api/users", "")
	key2 := GenerateCacheKey("GET", "/api/users", "")
	key3 := GenerateCacheKey("GET", "/api/products", "")

	if key1 != key2 {
		t.Error("same requests should have same cache key")
	}

	if key1 == key3 {
		t.Error("different requests should have different cache keys")
	}
}

func TestCacheableResponse(t *testing.T) {
	tests := []struct {
		status int
		header http.Header
		want   bool
	}{
		{200, http.Header{}, true},
		{404, http.Header{}, true},
		{500, http.Header{}, false},
		{200, http.Header{"Cache-Control": {"no-cache"}}, false},
		{200, http.Header{"Cache-Control": {"no-store"}}, false},
	}

	for _, tt := range tests {
		got := CacheableResponse(tt.status, tt.header)
		if got != tt.want {
			t.Errorf("CacheableResponse(%d, ...) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestExtractCacheTTL(t *testing.T) {
	tests := []struct {
		header http.Header
		minTTL time.Duration
		maxTTL time.Duration
	}{
		{http.Header{"Cache-Control": {"max-age=300"}}, 250 * time.Second, 350 * time.Second},
		{http.Header{}, 4*time.Minute + 50*time.Second, 5*time.Minute + 10*time.Second},
	}

	for i, tt := range tests {
		got := ExtractCacheTTL(tt.header)
		if got < tt.minTTL || got > tt.maxTTL {
			t.Errorf("test %d: ExtractCacheTTL got %v, want between %v and %v", i, got, tt.minTTL, tt.maxTTL)
		}
	}
}
