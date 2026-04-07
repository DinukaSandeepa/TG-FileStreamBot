package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrMalformedToken  = errors.New("malformed token")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrTokenExpired    = errors.New("token expired")
	ErrIDMismatch      = errors.New("token id mismatch")
)

type StreamTokenClaims struct {
	ID       string `json:"id"`
	Download bool   `json:"d,omitempty"`
	Exp      int64  `json:"exp"`
}

func SignStreamToken(id string, download bool, ttl time.Duration, secret string, now time.Time) (string, error) {
	if id == "" {
		return "", errors.New("id is required")
	}
	if secret == "" {
		return "", errors.New("secret is required")
	}
	if ttl <= 0 {
		return "", errors.New("ttl must be greater than zero")
	}

	claims := StreamTokenClaims{
		ID:       id,
		Download: download,
		Exp:      now.Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadPart))
	sig := mac.Sum(nil)
	sigPart := base64.RawURLEncoding.EncodeToString(sig)

	return payloadPart + "." + sigPart, nil
}

func VerifyStreamToken(token, expectedID, secret string, now time.Time) (*StreamTokenClaims, error) {
	if token == "" || secret == "" {
		return nil, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrMalformedToken
	}
	payloadPart, sigPart := parts[0], parts[1]

	providedSig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return nil, ErrMalformedToken
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadPart))
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(providedSig, expectedSig) {
		return nil, ErrInvalidSignature
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, ErrMalformedToken
	}
	claims := new(StreamTokenClaims)
	if err := json.Unmarshal(payloadJSON, claims); err != nil {
		return nil, ErrMalformedToken
	}

	if claims.ID == "" || claims.Exp == 0 {
		return nil, ErrMalformedToken
	}
	if expectedID != "" && claims.ID != expectedID {
		return nil, ErrIDMismatch
	}
	if now.Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

func BuildStreamDBURL(host, id, token string) string {
	h := strings.TrimRight(host, "/")
	return fmt.Sprintf("%s/stream/db/%s?token=%s", h, id, token)
}
