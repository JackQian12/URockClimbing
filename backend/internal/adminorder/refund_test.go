package adminorder

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"urockclimbing.com/backend/internal/platform/wechat"
)

type rejectingRefundGateway struct{}

func (rejectingRefundGateway) CreateRefund(context.Context, wechat.RefundInput) (wechat.RefundResult, error) {
	return wechat.RefundResult{}, errors.New("not implemented")
}
func (rejectingRefundGateway) QueryRefund(context.Context, string) (wechat.RefundResult, error) {
	return wechat.RefundResult{}, errors.New("not implemented")
}
func (rejectingRefundGateway) ParseRefundNotification(context.Context, *http.Request) (wechat.RefundResult, error) {
	return wechat.RefundResult{}, errors.New("signature invalid")
}

func TestRefundNotifyRejectsUnverifiedRequestBeforeDatabaseMutation(t *testing.T) {
	handler := &Handler{refunds: rejectingRefundGateway{}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/refunds/wechat/notify", nil)
	response := httptest.NewRecorder()

	handler.RefundNotify(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Body.String() != "{\"code\":\"FAIL\",\"message\":\"invalid notification\"}\n" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
