package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const accessTokenTTL = 15 * time.Minute
const refreshTokenTTL = 30 * 24 * time.Hour

type accessClaims struct {
	Subject string `json:"sub"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
}

func issueAccessToken(secret []byte, userID uint64, now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, err := json.Marshal(accessClaims{
		Subject: strconv.FormatUint(userID, 10),
		Issued:  now.Unix(),
		Expires: now.Add(accessTokenTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	signature := signToken(secret, unsigned)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseAccessToken(secret []byte, token string, now time.Time) (uint64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("invalid access token")
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, signToken(secret, unsigned)) {
		return 0, errors.New("invalid access token signature")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid access token header")
	}
	var header map[string]string
	if json.Unmarshal(headerJSON, &header) != nil || header["alg"] != "HS256" || header["typ"] != "JWT" {
		return 0, errors.New("invalid access token algorithm")
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, errors.New("invalid access token claims")
	}
	var claims accessClaims
	if json.Unmarshal(claimsJSON, &claims) != nil || claims.Subject == "" || claims.Expires <= now.Unix() || claims.Issued > now.Add(time.Minute).Unix() {
		return 0, errors.New("expired or invalid access token")
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return 0, errors.New("invalid access token subject")
	}
	return userID, nil
}

func signToken(secret []byte, value string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func randomToken() (plain, hash string, err error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buffer)
	return plain, hashToken(plain), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}
