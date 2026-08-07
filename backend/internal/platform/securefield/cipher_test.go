package securefield

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPhoneRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	cipher, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("13800138000")
	if err != nil {
		t.Fatal(err)
	}
	if string(encrypted) == "13800138000" {
		t.Fatal("phone stored as plaintext")
	}
	phone, err := cipher.Decrypt(encrypted)
	if err != nil || phone != "13800138000" {
		t.Fatalf("phone = %q, err = %v", phone, err)
	}
}

func TestInvalidKey(t *testing.T) {
	if _, err := New("not-a-key"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
