package admincard

import "testing"

func TestValidateInput(t *testing.T) {
	totalTimes := uint(10)
	input := productInput{
		Name: "10 次攀岩卡", ProductType: "COUNT_CARD", TotalTimes: &totalTimes, ValidityDays: 365, ActivationMode: "PURCHASE",
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

func TestValidateTimePass(t *testing.T) {
	input := productInput{
		Name: "月卡", ProductType: "TIME_PASS", ValidityDays: 30, ActivationMode: "FIRST_USE",
		PriceCent: 49900, DailyUseLimit: 1, ThemeColor: "#6F4A2E",
	}
	if message := validateInput(input, false); message != "" {
		t.Fatalf("valid time pass rejected: %s", message)
	}
	totalTimes := uint(30)
	input.TotalTimes = &totalTimes
	if message := validateInput(input, false); message != "期限卡不限总次数" {
		t.Fatalf("unexpected total times validation: %q", message)
	}
	input.TotalTimes = nil
	input.ValidityDays = 31
	if message := validateInput(input, false); message != "期限卡只支持 7、30、90 或 365 天" {
		t.Fatalf("unexpected validity validation: %q", message)
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
