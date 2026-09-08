package wechat

import (
	"context"
	"errors"
	"net/http"
	"time"

	wechatcore "github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// PayParameters are the exact fields accepted by wx.requestPayment.
type PayParameters struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type PayOrder struct {
	AppID         string
	MerchantID    string
	OrderNo       string
	TransactionID string
	TradeState    string
	Currency      string
	AmountCent    int64
	SuccessTime   time.Time
}

type PrepayInput struct {
	OrderNo     string
	OpenID      string
	Description string
	AmountCent  int64
	ExpiresAt   time.Time
}

type PaymentGateway interface {
	Prepay(context.Context, PrepayInput) (PayParameters, error)
	Query(context.Context, string) (PayOrder, error)
	ParsePaymentNotification(context.Context, *http.Request) (PayOrder, error)
}

type RefundInput struct {
	RefundNo      string
	OrderNo       string
	TransactionID string
	Reason        string
	AmountCent    int64
}

type RefundResult struct {
	MerchantID       string
	RefundNo         string
	OrderNo          string
	TransactionID    string
	ProviderRefundID string
	Status           string
	Currency         string
	TotalCent        int64
	RefundCent       int64
	SuccessTime      time.Time
}

type RefundRejectedError struct{ Code string }

func (e *RefundRejectedError) Error() string {
	return "wechat pay explicitly rejected refund: " + e.Code
}

type RefundGateway interface {
	CreateRefund(context.Context, RefundInput) (RefundResult, error)
	QueryRefund(context.Context, string) (RefundResult, error)
	ParseRefundNotification(context.Context, *http.Request) (RefundResult, error)
}

type PayRefundGateway interface {
	PaymentGateway
	RefundGateway
}

type PaymentConfig struct {
	AppID           string
	MerchantID      string
	APIv3Key        string
	CertSerialNo    string
	PrivateKeyPath  string
	PublicKeyID     string
	PublicKeyPath   string
	NotifyURL       string
	RefundNotifyURL string
}

type PaymentClient struct {
	appID           string
	merchantID      string
	notifyURL       string
	refundNotifyURL string
	service         jsapi.JsapiApiService
	refunds         refunddomestic.RefundsApiService
	notify          *notify.Handler
}

func NewPaymentClient(ctx context.Context, cfg PaymentConfig) (*PaymentClient, error) {
	privateKey, err := utils.LoadPrivateKeyWithPath(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, err := utils.LoadPublicKeyWithPath(cfg.PublicKeyPath)
	if err != nil {
		return nil, err
	}
	client, err := wechatcore.NewClient(ctx,
		option.WithWechatPayPublicKeyAuthCipher(cfg.MerchantID, cfg.CertSerialNo, privateKey, cfg.PublicKeyID, publicKey),
		option.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
	if err != nil {
		return nil, err
	}
	notifyHandler, err := notify.NewRSANotifyHandler(cfg.APIv3Key, verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.PublicKeyID, *publicKey))
	if err != nil {
		return nil, err
	}
	return &PaymentClient{
		appID: cfg.AppID, merchantID: cfg.MerchantID, notifyURL: cfg.NotifyURL, refundNotifyURL: cfg.RefundNotifyURL,
		service: jsapi.JsapiApiService{Client: client}, refunds: refunddomestic.RefundsApiService{Client: client}, notify: notifyHandler,
	}, nil
}

func (c *PaymentClient) CreateRefund(ctx context.Context, input RefundInput) (RefundResult, error) {
	response, _, err := c.refunds.Create(ctx, refunddomestic.CreateRequest{
		TransactionId: wechatcore.String(input.TransactionID),
		OutRefundNo:   wechatcore.String(input.RefundNo),
		Reason:        wechatcore.String(input.Reason),
		NotifyUrl:     wechatcore.String(c.refundNotifyURL),
		Amount:        &refunddomestic.AmountReq{Refund: wechatcore.Int64(input.AmountCent), Total: wechatcore.Int64(input.AmountCent), Currency: wechatcore.String("CNY")},
	})
	if err != nil {
		var apiError *wechatcore.APIError
		if errors.As(err, &apiError) && explicitRefundRejection(apiError.Code) {
			return RefundResult{}, &RefundRejectedError{Code: apiError.Code}
		}
		return RefundResult{}, err
	}
	return mapRefund(response, c.merchantID)
}

func explicitRefundRejection(code string) bool {
	switch code {
	case "USER_ACCOUNT_ABNORMAL", "NOT_ENOUGH", "PARAM_ERROR", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS", "SIGN_ERROR", "INVALID_REQUEST", "NO_AUTH":
		return true
	default:
		return false
	}
}

func (c *PaymentClient) QueryRefund(ctx context.Context, refundNo string) (RefundResult, error) {
	response, _, err := c.refunds.QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{OutRefundNo: wechatcore.String(refundNo)})
	if err != nil {
		return RefundResult{}, err
	}
	return mapRefund(response, c.merchantID)
}

func (c *PaymentClient) ParseRefundNotification(ctx context.Context, request *http.Request) (RefundResult, error) {
	resource := new(struct {
		MerchantID       *string `json:"mchid"`
		OrderNo          *string `json:"out_trade_no"`
		TransactionID    *string `json:"transaction_id"`
		RefundNo         *string `json:"out_refund_no"`
		ProviderRefundID *string `json:"refund_id"`
		Status           *string `json:"refund_status"`
		SuccessTime      *string `json:"success_time"`
		Amount           *struct {
			Total    *int64  `json:"total"`
			Refund   *int64  `json:"refund"`
			Currency *string `json:"currency"`
		} `json:"amount"`
	})
	if _, err := c.notify.ParseNotifyRequest(ctx, request, resource); err != nil {
		return RefundResult{}, err
	}
	if resource.MerchantID == nil || resource.OrderNo == nil || resource.TransactionID == nil || resource.RefundNo == nil || resource.ProviderRefundID == nil || resource.Status == nil || resource.Amount == nil || resource.Amount.Total == nil || resource.Amount.Refund == nil {
		return RefundResult{}, errors.New("wechat pay returned an incomplete refund notification")
	}
	result := RefundResult{MerchantID: *resource.MerchantID, RefundNo: *resource.RefundNo, OrderNo: *resource.OrderNo, TransactionID: *resource.TransactionID, ProviderRefundID: *resource.ProviderRefundID, Status: *resource.Status, TotalCent: *resource.Amount.Total, RefundCent: *resource.Amount.Refund, Currency: "CNY"}
	if resource.Amount.Currency != nil {
		result.Currency = *resource.Amount.Currency
	}
	if resource.SuccessTime != nil && *resource.SuccessTime != "" {
		parsed, err := time.Parse(time.RFC3339, *resource.SuccessTime)
		if err != nil {
			return RefundResult{}, err
		}
		result.SuccessTime = parsed
	}
	return result, nil
}

func mapRefund(refund *refunddomestic.Refund, merchantID string) (RefundResult, error) {
	if refund == nil || refund.RefundId == nil || refund.OutRefundNo == nil || refund.TransactionId == nil || refund.OutTradeNo == nil || refund.Status == nil || refund.Amount == nil || refund.Amount.Total == nil || refund.Amount.Refund == nil || refund.Amount.Currency == nil {
		return RefundResult{}, errors.New("wechat pay returned an incomplete refund")
	}
	result := RefundResult{MerchantID: merchantID, RefundNo: *refund.OutRefundNo, OrderNo: *refund.OutTradeNo, TransactionID: *refund.TransactionId, ProviderRefundID: *refund.RefundId, Status: string(*refund.Status), Currency: *refund.Amount.Currency, TotalCent: *refund.Amount.Total, RefundCent: *refund.Amount.Refund}
	if refund.SuccessTime != nil {
		result.SuccessTime = refund.SuccessTime.UTC()
	}
	return result, nil
}

func (c *PaymentClient) Prepay(ctx context.Context, input PrepayInput) (PayParameters, error) {
	response, _, err := c.service.PrepayWithRequestPayment(ctx, jsapi.PrepayRequest{
		Appid:       wechatcore.String(c.appID),
		Mchid:       wechatcore.String(c.merchantID),
		Description: wechatcore.String(input.Description),
		OutTradeNo:  wechatcore.String(input.OrderNo),
		TimeExpire:  &input.ExpiresAt,
		NotifyUrl:   wechatcore.String(c.notifyURL),
		Amount: &jsapi.Amount{
			Total:    wechatcore.Int64(input.AmountCent),
			Currency: wechatcore.String("CNY"),
		},
		Payer: &jsapi.Payer{Openid: wechatcore.String(input.OpenID)},
	})
	if err != nil {
		return PayParameters{}, err
	}
	if response == nil || response.TimeStamp == nil || response.NonceStr == nil || response.Package == nil || response.SignType == nil || response.PaySign == nil {
		return PayParameters{}, errors.New("wechat pay returned incomplete requestPayment parameters")
	}
	return PayParameters{TimeStamp: *response.TimeStamp, NonceStr: *response.NonceStr, Package: *response.Package, SignType: *response.SignType, PaySign: *response.PaySign}, nil
}

func (c *PaymentClient) Query(ctx context.Context, orderNo string) (PayOrder, error) {
	transaction, _, err := c.service.QueryOrderByOutTradeNo(ctx, jsapi.QueryOrderByOutTradeNoRequest{
		OutTradeNo: wechatcore.String(orderNo), Mchid: wechatcore.String(c.merchantID),
	})
	if err != nil {
		return PayOrder{}, err
	}
	return mapTransaction(transaction)
}

func (c *PaymentClient) ParsePaymentNotification(ctx context.Context, request *http.Request) (PayOrder, error) {
	transaction := new(struct {
		AppID         *string `json:"appid"`
		MerchantID    *string `json:"mchid"`
		OrderNo       *string `json:"out_trade_no"`
		TransactionID *string `json:"transaction_id"`
		TradeState    *string `json:"trade_state"`
		SuccessTime   *string `json:"success_time"`
		Amount        *struct {
			Total    *int64  `json:"total"`
			Currency *string `json:"currency"`
		} `json:"amount"`
	})
	if _, err := c.notify.ParseNotifyRequest(ctx, request, transaction); err != nil {
		return PayOrder{}, err
	}
	var total *int64
	var currency *string
	if transaction.Amount != nil {
		total, currency = transaction.Amount.Total, transaction.Amount.Currency
	}
	return normalizedPayOrder(transaction.AppID, transaction.MerchantID, transaction.OrderNo, transaction.TransactionID, transaction.TradeState, transaction.SuccessTime, total, currency)
}

func mapTransaction(transaction *payments.Transaction) (PayOrder, error) {
	if transaction == nil {
		return PayOrder{}, errors.New("wechat pay returned an empty transaction")
	}
	var total *int64
	var currency *string
	if transaction.Amount != nil {
		total, currency = transaction.Amount.Total, transaction.Amount.Currency
	}
	return normalizedPayOrder(transaction.Appid, transaction.Mchid, transaction.OutTradeNo, transaction.TransactionId, transaction.TradeState, transaction.SuccessTime, total, currency)
}

func normalizedPayOrder(appID, merchantID, orderNo, transactionID, tradeState, successTime *string, total *int64, currency *string) (PayOrder, error) {
	if appID == nil || merchantID == nil || orderNo == nil || tradeState == nil || total == nil || currency == nil {
		return PayOrder{}, errors.New("wechat pay returned an incomplete transaction")
	}
	result := PayOrder{AppID: *appID, MerchantID: *merchantID, OrderNo: *orderNo, TradeState: *tradeState, AmountCent: *total, Currency: *currency}
	if transactionID != nil {
		result.TransactionID = *transactionID
	}
	if successTime != nil && *successTime != "" {
		parsed, err := time.Parse(time.RFC3339, *successTime)
		if err != nil {
			return PayOrder{}, err
		}
		result.SuccessTime = parsed
	}
	return result, nil
}
