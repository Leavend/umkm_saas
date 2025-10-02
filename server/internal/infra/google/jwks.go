package google

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type Verifier struct {
	issuer     string
	clientID   string
	mu         sync.RWMutex
	cache      map[string]*rsa.PublicKey
	fetched    time.Time
	httpClient *http.Client
}

func NewVerifier(issuer, clientID string) *Verifier {
	return &Verifier{
		issuer:     issuer,
		clientID:   clientID,
		cache:      make(map[string]*rsa.PublicKey),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *Verifier) VerifyIDToken(ctx context.Context, token string) (map[string]any, error) {
	header, payload, signature, signingInput, err := parseJWT(token)
	if err != nil {
		return nil, err
	}
	if err := v.ensureKeys(ctx); err != nil {
		return nil, err
	}
	kid, _ := header["kid"].(string)
	key, ok := v.keyFor(kid)
	if !ok {
		if err := v.refresh(ctx); err != nil {
			return nil, err
		}
		key, ok = v.keyFor(kid)
		if !ok {
			return nil, errors.New("unknown kid")
		}
	}
	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed[:], signature); err != nil {
		return nil, err
	}
	if iss, _ := payload["iss"].(string); iss != v.issuer {
		return nil, errors.New("invalid issuer")
	}
	if aud, _ := payload["aud"].(string); aud != v.clientID {
		return nil, errors.New("invalid audience")
	}
	if exp, ok := payload["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	}
	return payload, nil
}

func (v *Verifier) ensureKeys(ctx context.Context) error {
	v.mu.RLock()
	fresh := time.Since(v.fetched) < time.Hour && len(v.cache) > 0
	v.mu.RUnlock()
	if fresh {
		return nil
	}
	return v.refresh(ctx)
}

func (v *Verifier) refresh(ctx context.Context) error {
	cfg, err := v.fetchConfig(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURI, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var set jwks
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, key := range set.Keys {
		if key.Kty != "RSA" {
			continue
		}
		pub, err := rsaKeyFromJWK(key)
		if err != nil {
			continue
		}
		keys[key.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("no keys fetched")
	}
	v.mu.Lock()
	v.cache = keys
	v.fetched = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *Verifier) fetchConfig(ctx context.Context) (*struct {
	JWKSURI string `json:"jwks_uri"`
}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cfg struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (v *Verifier) keyFor(kid string) (*rsa.PublicKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	pk, ok := v.cache[kid]
	return pk, ok
}

func rsaKeyFromJWK(j jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func parseJWT(token string) (map[string]any, map[string]any, []byte, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, nil, "", errors.New("invalid token")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, "", err
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, "", err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, nil, "", err
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, nil, nil, "", err
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, nil, nil, "", err
	}
	return header, payload, signature, parts[0] + "." + parts[1], nil
}
