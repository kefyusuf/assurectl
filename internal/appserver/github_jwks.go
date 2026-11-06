package appserver

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
)

// jwksKeySource implements KeySource by fetching and caching the issuer's
// JWKS document; an unknown kid forces exactly one refresh per loaded
// document (key rotation).
type jwksKeySource struct {
	wellKnown string
	client    *http.Client

	mu                 sync.Mutex
	refreshedAfterLoad bool
	keys               map[string]*rsa.PublicKey
}

func (s *jwksKeySource) Key(kid string) (interface{}, error) {
	s.mu.Lock()
	key, ok := s.keys[kid]
	canRefresh := !s.refreshedAfterLoad
	s.mu.Unlock()
	if ok {
		return key, nil
	}
	if !canRefresh {
		return nil, fmt.Errorf("jwks: key %q not found", kid)
	}

	if err := s.fetch(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	key, ok = s.keys[kid]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("jwks: key %q not found after refresh", kid)
	}
	return key, nil
}

func (s *jwksKeySource) fetch() error {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.wellKnown, nil)
	if err != nil {
		return fmt.Errorf("jwks: %w", err)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("jwks: fetch: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: fetch failed with status %d", response.StatusCode)
	}

	var document struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return fmt.Errorf("jwks: decode: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, key := range document.Keys {
		if key.Kty != "RSA" {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return fmt.Errorf("jwks: key %q modulus: %w", key.Kid, err)
		}
		exponent, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			return fmt.Errorf("jwks: key %q exponent: %w", key.Kid, err)
		}
		keys[key.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(modulus),
			E: int(new(big.Int).SetBytes(exponent).Int64()),
		}
	}

	s.mu.Lock()
	s.keys = keys
	s.refreshedAfterLoad = true
	s.mu.Unlock()
	return nil
}
