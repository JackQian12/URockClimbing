package order

import (
	"regexp"
	"testing"
)

func TestOrderNumberMeetsWechatPayConstraints(t *testing.T) {
	orderNo, err := randomBusinessNo("URORD")
	if err != nil {
		t.Fatal(err)
	}
	if len(orderNo) < 6 || len(orderNo) > 32 {
		t.Fatalf("order number length = %d, want 6..32", len(orderNo))
	}
	if !regexp.MustCompile(`^[0-9A-Za-z_\-|*]+$`).MatchString(orderNo) {
		t.Fatalf("order number contains unsupported characters: %q", orderNo)
	}
}
