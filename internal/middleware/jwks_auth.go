package middleware

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWKSClient fetches and caches public keys from a JWKS endpoint.
type JWKSClient struct {
	endpoint  string
	cache     map[string]*rsa.PublicKey
	mu        sync.RWMutex
	ttl       time.Duration
	lastFetch time.Time
}

// JWKS represents the JSON Web Key Set format.
type JWKS struct {
	Keys []struct {
		Kty string `json:"kty"` // Key type (RSA, EC, etc.)
		Use string `json:"use"` // Key usage (sig, enc, etc.)
		Kid string `json:"kid"` // Key ID
		N   string `json:"n"`   // Modulus (RSA)
		E   string `json:"e"`   // Exponent (RSA)
	} `json:"keys"`
}

// NewJWKSClient creates a client that fetches keys from a JWKS endpoint.
func NewJWKSClient(endpoint string, ttl time.Duration) *JWKSClient {
	return &JWKSClient{
		endpoint: endpoint,
		cache:    make(map[string]*rsa.PublicKey),
		ttl:      ttl,
	}
}

// GetPublicKey fetches a public key by kid from cache or remote endpoint.
func (c *JWKSClient) GetPublicKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	if key, ok := c.cache[kid]; ok && time.Since(c.lastFetch) < c.ttl {
		c.mu.RUnlock()
		return key, nil
	}
	c.mu.RUnlock()

	// Fetch fresh keys
	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("failed to refresh JWKS: %w", err)
	}

	c.mu.RLock()
	key, ok := c.cache[kid]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}
	return key, nil
}

func (c *JWKSClient) refresh() error {
	resp, err := http.Get(c.endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("JWKS endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to parse JWKS: %w", err)
	}

	cache := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Use != "sig" {
			continue
		}
		pubKey, err := decodeRSAPublicKey(key.N, key.E)
		if err != nil {
			continue // Skip invalid keys
		}
		cache[key.Kid] = pubKey
	}

	c.mu.Lock()
	c.cache = cache
	c.lastFetch = time.Now()
	c.mu.Unlock()
	return nil
}

func decodeRSAPublicKey(n, e string) (*rsa.PublicKey, error) {
	parser := jwt.NewParser()
	nBytes, err := parser.DecodeSegment(n)
	if err != nil {
		return nil, err
	}
	eBytes, err := parser.DecodeSegment(e)
	if err != nil {
		return nil, err
	}

	var nInt big.Int
	nInt.SetBytes(nBytes)
	// e is typically 65537 (0x10001)
	eVal := 0
	for _, b := range eBytes {
		eVal = eVal*256 + int(b)
	}

	return &rsa.PublicKey{N: &nInt, E: eVal}, nil
}

// NewJWKSMiddleware returns a middleware that validates JWT tokens using JWKS.
// It checks signing method (RS256), expiration, issuer, and audience.
func NewJWKSMiddleware(jwksClient *JWKSClient, expectedIssuer, expectedAudience string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}
			parts := strings.Fields(auth)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}
			tokenStr := parts[1]

			// Parse token with claims
			var claims CustomClaims
			token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
				// Check signing method
				alg, ok := t.Header["alg"].(string)
				if !ok || alg != "RS256" {
					return nil, fmt.Errorf("unsupported signing method: %v", t.Header["alg"])
				}

				// Get the kid from header
				kid, ok := t.Header["kid"].(string)
				if !ok {
					return nil, fmt.Errorf("missing kid in token header")
				}

				// Fetch public key
				pubKey, err := jwksClient.GetPublicKey(kid)
				if err != nil {
					return nil, err
				}
				return pubKey, nil
			})
			if err != nil {
				writeUnauthorized(w, "invalid token: "+err.Error())
				return
			}
			if !token.Valid {
				writeUnauthorized(w, "invalid token")
				return
			}

			// Validate claims
			if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
				writeUnauthorized(w, "token is expired")
				return
			}
			if expectedIssuer != "" && claims.Issuer != expectedIssuer {
				writeUnauthorized(w, "invalid token issuer")
				return
			}
			if expectedAudience != "" {
				// Check if audience contains expectedAudience
				found := false
				for _, aud := range claims.Audience {
					if aud == expectedAudience {
						found = true
						break
					}
				}
				if !found {
					writeUnauthorized(w, "invalid token audience")
					return
				}
			}

			// Inject headers and continue
			r2 := r.Clone(r.Context())
			if claims.Subject != "" {
				r2.Header.Set("X-User-ID", claims.Subject)
			}
			if claims.Role != "" {
				r2.Header.Set("X-User-Role", claims.Role)
			}
			next.ServeHTTP(w, r2)
		})
	}
}
