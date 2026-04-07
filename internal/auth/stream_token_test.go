package auth

import (
	"errors"
	"testing"
	"time"
)

func TestSignAndVerifyStreamToken(t *testing.T) {
	secret := "test-secret"
	now := time.Unix(1_700_000_000, 0)
	token, err := SignStreamToken("68118db98f8f82c0e790c1fd", true, 10*time.Minute, secret, now)
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	claims, err := VerifyStreamToken(token, "68118db98f8f82c0e790c1fd", secret, now.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("unexpected verify error: %v", err)
	}
	if !claims.Download {
		t.Fatalf("expected download claim to be true")
	}
}

func TestVerifyStreamTokenExpired(t *testing.T) {
	secret := "test-secret"
	now := time.Unix(1_700_000_000, 0)
	token, err := SignStreamToken("68118db98f8f82c0e790c1fd", false, 1*time.Minute, secret, now)
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	_, err = VerifyStreamToken(token, "68118db98f8f82c0e790c1fd", secret, now.Add(2*time.Minute))
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyStreamTokenIDMismatch(t *testing.T) {
	secret := "test-secret"
	now := time.Unix(1_700_000_000, 0)
	token, err := SignStreamToken("68118db98f8f82c0e790c1fd", false, 10*time.Minute, secret, now)
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	_, err = VerifyStreamToken(token, "aaaaaaaaaaaaaaaaaaaaaaaa", secret, now.Add(10*time.Second))
	if !errors.Is(err, ErrIDMismatch) {
		t.Fatalf("expected ErrIDMismatch, got %v", err)
	}
}

func TestVerifyStreamTokenInvalidSignature(t *testing.T) {
	secret := "test-secret"
	now := time.Unix(1_700_000_000, 0)
	token, err := SignStreamToken("68118db98f8f82c0e790c1fd", false, 10*time.Minute, secret, now)
	if err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	token = token + "a"
	_, err = VerifyStreamToken(token, "68118db98f8f82c0e790c1fd", secret, now.Add(10*time.Second))
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}
