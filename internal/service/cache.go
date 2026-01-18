package service

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CacheEntry holds cached HTTP response data
type CacheEntry struct {
	Status    int
	Headers   http.Header
	Body      []byte
	ExpiresAt time.Time
	HitCount  int64
	CreatedAt time.Time
}

// IsExpired checks if cache entry has expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Now().After(ce.ExpiresAt)
}

// ResponseCache caches HTTP responses
type ResponseCache struct {
	mu       sync.RWMutex
	cache    map[string]*CacheEntry
	maxSize  int
	maxEntry int64
}

// NewResponseCache creates a new response cache
func NewResponseCache(maxSize int, maxEntrySize int64) *ResponseCache {
	rc := &ResponseCache{
		cache:    make(map[string]*CacheEntry),
		maxSize:  maxSize,
		maxEntry: maxEntrySize,
	}

	// Start cleanup goroutine
	go rc.cleanupExpired()

	return rc
}

// Get retrieves a cached response if it exists and isn't expired
func (rc *ResponseCache) Get(key string) (*CacheEntry, bool) {
	rc.mu.RLock()
	entry, exists := rc.cache[key]
	rc.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if entry.IsExpired() {
		rc.Delete(key)
		return nil, false
	}

	// Update hit count
	rc.mu.Lock()
	entry.HitCount++
	rc.mu.Unlock()

	return entry, true
}

// Set stores a response in the cache
func (rc *ResponseCache) Set(key string, entry *CacheEntry) {
	// Check entry size
	if int64(len(entry.Body)) > rc.maxEntry {
		return
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check cache size limit
	if len(rc.cache) >= rc.maxSize {
		// Evict least recently used (by hit count)
		rc.evictLRU()
	}

	rc.cache[key] = entry
}

// Delete removes a cache entry
func (rc *ResponseCache) Delete(key string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	delete(rc.cache, key)
}

// Clear removes all cache entries
func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[string]*CacheEntry)
}

// evictLRU evicts the least recently used entry
func (rc *ResponseCache) evictLRU() {
	var lruKey string
	var minHits int64 = int64(^uint64(0) >> 1) // max int64

	for key, entry := range rc.cache {
		if entry.HitCount < minHits {
			minHits = entry.HitCount
			lruKey = key
		}
	}

	if lruKey != "" {
		delete(rc.cache, lruKey)
	}
}

// cleanupExpired periodically removes expired entries
func (rc *ResponseCache) cleanupExpired() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rc.mu.Lock()
		for key, entry := range rc.cache {
			if entry.IsExpired() {
				delete(rc.cache, key)
			}
		}
		rc.mu.Unlock()
	}
}

// GetSize returns current cache size (number of entries)
func (rc *ResponseCache) GetSize() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return len(rc.cache)
}

// GenerateCacheKey generates a cache key from request
func GenerateCacheKey(method, path string, query string) string {
	key := fmt.Sprintf("%s:%s:%s", method, path, query)
	return fmt.Sprintf("%x", md5.Sum([]byte(key)))
}

// CacheableResponse checks if a response should be cached
func CacheableResponse(status int, headers http.Header) bool {
	// Only cache successful GET/HEAD responses
	// Check Cache-Control header
	cacheControl := headers.Get("Cache-Control")
	if cacheControl == "no-cache" || cacheControl == "no-store" {
		return false
	}

	return status == http.StatusOK || status == http.StatusNotFound
}

// ExtractCacheTTL extracts TTL from response headers
func ExtractCacheTTL(headers http.Header) time.Duration {
	// Check Cache-Control max-age
	cacheControl := headers.Get("Cache-Control")
	if cacheControl != "" {
		// Simple parsing - in production, use a proper parser
		var maxAge int
		fmt.Sscanf(cacheControl, "max-age=%d", &maxAge)
		if maxAge > 0 {
			return time.Duration(maxAge) * time.Second
		}
	}

	// Default TTL: 5 minutes
	return 5 * time.Minute
}

// CachedRoundTripper wraps http.RoundTripper with caching
type CachedRoundTripper struct {
	transport http.RoundTripper
	cache     *ResponseCache
}

// NewCachedRoundTripper creates a new cached round tripper
func NewCachedRoundTripper(cache *ResponseCache) *CachedRoundTripper {
	return &CachedRoundTripper{
		transport: http.DefaultTransport,
		cache:     cache,
	}
}

// RoundTrip implements http.RoundTripper
func (crt *CachedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only cache GET requests
	if req.Method != http.MethodGet {
		return crt.transport.RoundTrip(req)
	}

	// Check cache
	cacheKey := GenerateCacheKey(req.Method, req.URL.Path, req.URL.RawQuery)
	if cached, exists := crt.cache.Get(cacheKey); exists {
		// Return cached response
		return &http.Response{
			Status:     fmt.Sprintf("%d %s", cached.Status, http.StatusText(cached.Status)),
			StatusCode: cached.Status,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     cached.Headers,
			Body:       io.NopCloser(bytes.NewReader(cached.Body)),
			Request:    req,
			// Add cache hit header for debugging
		}, nil
	}

	// Execute request
	resp, err := crt.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Read response body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Cache if applicable
	if CacheableResponse(resp.StatusCode, resp.Header) {
		ttl := ExtractCacheTTL(resp.Header)
		entry := &CacheEntry{
			Status:    resp.StatusCode,
			Headers:   resp.Header.Clone(),
			Body:      body,
			ExpiresAt: time.Now().Add(ttl),
			CreatedAt: time.Now(),
		}
		crt.cache.Set(cacheKey, entry)
		resp.Header.Set("X-Cache", "MISS")
	}

	// Return response with new body
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}
