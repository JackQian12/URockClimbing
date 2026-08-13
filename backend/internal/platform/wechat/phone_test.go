package wechat

import (
	"context"
	"testing"
)

func TestMockPhoneNumberExchange(t *testing.T) {
	client := NewLoginClient("", "", true)
	phone, err := client.ExchangePhoneNumber(context.Background(), "mock-phone-code")
	if err != nil {
		t.Fatalf("exchange mock phone number: %v", err)
	}
	if phone.PureNumber != "13800000000" || phone.CountryCode != "86" {
		t.Fatalf("unexpected mock phone result: %#v", phone)
	}
}

func TestPhoneNumberExchangeRejectsEmptyCode(t *testing.T) {
	client := NewLoginClient("", "", true)
	if _, err := client.ExchangePhoneNumber(context.Background(), "  "); err == nil {
		t.Fatal("expected empty phone code to be rejected")
	}
}
