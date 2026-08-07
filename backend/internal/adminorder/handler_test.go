package adminorder

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestRefundEligibility(t *testing.T) {
	now := time.Now().UTC()
	paid := now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	base := Order{Status: "PAID", PaymentStatus: ptr("SUCCEEDED"), CardNo: ptr("C1"), CardStatus: ptr("ACTIVE"), PaidAt: &paid}
	if ok, reason := refundEligibility(base, now); !ok {
		t.Fatalf("eligible order rejected: %s", reason)
	}
	used := base
	used.RedemptionCount = 1
	if ok, _ := refundEligibility(used, now); ok {
		t.Fatal("redeemed card accepted")
	}
	refunded := base
	refunded.RefundStatus = ptr("PROCESSING")
	if ok, _ := refundEligibility(refunded, now); ok {
		t.Fatal("duplicate refund accepted")
	}
}

func TestOrderStatusAndSearchEscaping(t *testing.T) {
	if validOrderStatus("INVALID") || !validOrderStatus("PAID") {
		t.Fatal("status validation failed")
	}
	if got := escapeLike("A_10%!"); got != "A!_10!%!!" {
		t.Fatalf("escapeLike=%q", got)
	}
}
