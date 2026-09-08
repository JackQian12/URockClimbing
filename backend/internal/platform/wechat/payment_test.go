package wechat

import "testing"

func TestExplicitRefundRejectionClassification(t *testing.T) {
	for _, code := range []string{"NOT_ENOUGH", "PARAM_ERROR", "RESOURCE_NOT_EXISTS", "NO_AUTH"} {
		if !explicitRefundRejection(code) {
			t.Fatalf("%s should be explicit", code)
		}
	}
	for _, code := range []string{"SYSTEM_ERROR", "FREQUENCY_LIMITED", ""} {
		if explicitRefundRejection(code) {
			t.Fatalf("%s must remain unknown for reconciliation", code)
		}
	}
}
