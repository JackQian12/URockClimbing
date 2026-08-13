package auth

import (
	"strings"
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Unix(1_700_000_000, 0).UTC()
	token, err := issueAccessToken(secret, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := parseAccessToken(secret, token, now.Add(time.Minute))
	if err != nil || userID != 42 {
		t.Fatalf("userID=%d err=%v", userID, err)
	}
}

func TestAccessTokenRejectsTamperingAndExpiry(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Unix(1_700_000_000, 0).UTC()
	token, err := issueAccessToken(secret, 42, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseAccessToken(secret, token+"x", now); err == nil {
		t.Fatal("tampered token accepted")
	}
	if _, err := parseAccessToken(secret, token, now.Add(accessTokenTTL+time.Second)); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestRefreshTokensAreRandomAndOnlyHashesPersist(t *testing.T) {
	first, firstHash, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("refresh tokens are not unique")
	}
	if first == firstHash || len(firstHash) != 64 {
		t.Fatal("refresh token hash is invalid")
	}
}
