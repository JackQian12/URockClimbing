package admincard

import "testing"

func TestValidateInput(t *testing.T) {
	input := productInput{
		Name: "10 次攀岩卡", TotalTimes: 10, ValidityDays: 365, ActivationMode: "PURCHASE",
		PriceCent: 168000, DailyUseLimit: 1, ThemeColor: "#6F4A2E", Version: 1,
	}
	if message := validateInput(input, true); message != "" {
		t.Fatalf("valid input rejected: %s", message)
	}
	listPrice := uint64(100000)
	input.ListPriceCent = &listPrice
	if message := validateInput(input, true); message != "划线价不能低于售价" {
		t.Fatalf("unexpected validation message: %q", message)
	}
}

func TestValidStatusTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"DRAFT", "ON_SALE", true},
		{"ON_SALE", "OFF_SALE", true},
		{"OFF_SALE", "ON_SALE", true},
		{"DRAFT", "OFF_SALE", false},
		{"ON_SALE", "DRAFT", false},
	}
	for _, test := range tests {
		if got := validStatusTransition(test.from, test.to); got != test.want {
			t.Errorf("%s -> %s = %v, want %v", test.from, test.to, got, test.want)
		}
	}
}
