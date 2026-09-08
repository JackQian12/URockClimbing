package payment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"urockclimbing.com/backend/internal/platform/wechat"
)

type rejectingGateway struct{}

func (rejectingGateway) Prepay(context.Context, wechat.PrepayInput) (wechat.PayParameters, error) {
	return wechat.PayParameters{}, errors.New("not implemented")
}
func (rejectingGateway) Query(context.Context, string) (wechat.PayOrder, error) {
	return wechat.PayOrder{}, errors.New("not implemented")
}
func (rejectingGateway) ParsePaymentNotification(context.Context, *http.Request) (wechat.PayOrder, error) {
	return wechat.PayOrder{}, errors.New("signature invalid")
}

func TestNotifyRejectsUnverifiedRequestBeforeDatabaseMutation(t *testing.T) {
	handler := &Handler{gateway: rejectingGateway{}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/payments/wechat/notify", nil)
	response := httptest.NewRecorder()

	handler.Notify(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Body.String() != "{\"code\":\"FAIL\",\"message\":\"invalid notification\"}\n" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
