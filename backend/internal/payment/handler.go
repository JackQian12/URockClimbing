package payment

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/order"
	"urockclimbing.com/backend/internal/platform/wechat"
	"urockclimbing.com/backend/internal/respond"
)

var (
	errOrderNotFound     = errors.New("order not found")
	errOrderNotPayable   = errors.New("order not payable")
	errPaymentMismatch   = errors.New("payment does not match order")
	errPaymentIncomplete = errors.New("payment is incomplete")
)

type Handler struct {
	db         *sql.DB
	auth       *auth.Handler
	gateway    wechat.PaymentGateway
	appID      string
	merchantID string
	now        func() time.Time
}

type payableOrder struct {
	ID         uint64
	OrderNo    string
	Status     string
	AmountCent uint64
	ExpiresAt  time.Time
	Snapshot   order.ProductSnapshot
	OpenID     string
}

func NewHandler(db *sql.DB, authenticator *auth.Handler, gateway wechat.PaymentGateway, appID, merchantID string) *Handler {
	return &Handler{db: db, auth: authenticator, gateway: gateway, appID: appID, merchantID: merchantID, now: time.Now}
}

func (h *Handler) Prepare(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	if h.gateway == nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "WECHAT_PAY_NOT_CONFIGURED", "微信支付配置尚未完成，请稍后再试")
		return
	}
	item, err := h.loadPayableOrder(r.Context(), user.ID, strings.TrimSpace(r.PathValue("order_no")))
	if errors.Is(err, errOrderNotFound) {
		respond.Error(w, r, http.StatusNotFound, "ORDER_NOT_FOUND", "订单不存在")
		return
	}
	if errors.Is(err, errOrderNotPayable) {
		respond.Error(w, r, http.StatusConflict, "ORDER_NOT_PAYABLE", "该订单当前不能支付")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "订单读取失败")
		return
	}
	if err := h.ensurePaymentTransaction(r.Context(), item); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "PAYMENT_CREATE_FAILED", "支付单创建失败，请稍后重试")
		return
	}
	description := "U-Rock遇岩-" + item.Snapshot.Name
	if len([]rune(description)) > 127 {
		description = string([]rune(description)[:127])
	}
	parameters, err := h.gateway.Prepay(r.Context(), wechat.PrepayInput{
		OrderNo: item.OrderNo, OpenID: item.OpenID, Description: description,
		AmountCent: int64(item.AmountCent), ExpiresAt: item.ExpiresAt,
	})
	if err != nil {
		respond.Error(w, r, http.StatusBadGateway, "WECHAT_PREPAY_FAILED", "微信支付下单失败，请稍后重试")
		return
	}
	respond.JSON(w, r, http.StatusOK, parameters)
}

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	if h.gateway == nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "WECHAT_PAY_NOT_CONFIGURED", "微信支付配置尚未完成")
		return
	}
	orderNo := strings.TrimSpace(r.PathValue("order_no"))
	var exists bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM orders WHERE user_id=? AND order_no=?)`, user.ID, orderNo).Scan(&exists); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单读取失败")
		return
	}
	if !exists {
		respond.Error(w, r, 404, "ORDER_NOT_FOUND", "订单不存在")
		return
	}
	result, err := h.gateway.Query(r.Context(), orderNo)
	if err != nil {
		respond.Error(w, r, http.StatusBadGateway, "WECHAT_QUERY_FAILED", "支付结果查询失败，请稍后刷新")
		return
	}
	if result.TradeState == "SUCCESS" {
		if err := h.settlePayment(r.Context(), result); err != nil {
			respond.Error(w, r, http.StatusConflict, "PAYMENT_SYNC_FAILED", "支付结果校验失败，请联系门店")
			return
		}
	}
	respond.JSON(w, r, http.StatusOK, map[string]string{"trade_state": result.TradeState})
}

func (h *Handler) Notify(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		writeNotifyResponse(w, http.StatusServiceUnavailable, "FAIL", "payment is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	result, err := h.gateway.ParsePaymentNotification(r.Context(), r)
	if err != nil {
		writeNotifyResponse(w, http.StatusBadRequest, "FAIL", "invalid notification")
		return
	}
	if result.TradeState != "SUCCESS" {
		writeNotifyResponse(w, http.StatusBadRequest, "FAIL", "unexpected trade state")
		return
	}
	if err := h.settlePayment(r.Context(), result); err != nil {
		writeNotifyResponse(w, http.StatusInternalServerError, "FAIL", "payment was not settled")
		return
	}
	writeNotifyResponse(w, http.StatusOK, "SUCCESS", "成功")
}

func (h *Handler) registeredUser(w http.ResponseWriter, r *http.Request) (auth.CurrentUser, bool) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, 401, "UNAUTHENTICATED", "请先登录")
		return auth.CurrentUser{}, false
	}
	if !user.Registered {
		respond.Error(w, r, 403, "REGISTRATION_REQUIRED", "请授权手机号完成会员注册")
		return auth.CurrentUser{}, false
	}
	return user, true
}

func (h *Handler) loadPayableOrder(ctx context.Context, userID uint64, orderNo string) (payableOrder, error) {
	if orderNo == "" || len(orderNo) > 64 {
		return payableOrder{}, errOrderNotFound
	}
	var item payableOrder
	var snapshot []byte
	err := h.db.QueryRowContext(ctx, `
		SELECT o.id,o.order_no,o.status,o.total_amount_cent,o.expires_at,o.product_snapshot,wi.openid
		FROM orders o JOIN wechat_identities wi ON wi.user_id=o.user_id AND wi.appid=?
		WHERE o.user_id=? AND o.order_no=?`, h.appID, userID, orderNo).
		Scan(&item.ID, &item.OrderNo, &item.Status, &item.AmountCent, &item.ExpiresAt, &snapshot, &item.OpenID)
	if errors.Is(err, sql.ErrNoRows) {
		return payableOrder{}, errOrderNotFound
	}
	if err != nil {
		return payableOrder{}, err
	}
	if item.Status != "PENDING" || !item.ExpiresAt.After(h.now().UTC()) {
		if item.Status == "PENDING" {
			_, _ = h.db.ExecContext(ctx, `UPDATE orders SET status='CLOSED',closed_at=UTC_TIMESTAMP(3) WHERE id=? AND status='PENDING'`, item.ID)
		}
		return payableOrder{}, errOrderNotPayable
	}
	if err := json.Unmarshal(snapshot, &item.Snapshot); err != nil {
		return payableOrder{}, err
	}
	return item, nil
}

func (h *Handler) ensurePaymentTransaction(ctx context.Context, item payableOrder) error {
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO payment_transactions(order_id,provider,merchant_order_no,status,amount_cent)
		VALUES(?,'WECHAT_PAY',?,'CREATED',?)
		ON DUPLICATE KEY UPDATE order_id=IF(order_id=VALUES(order_id),order_id,NULL),amount_cent=IF(amount_cent=VALUES(amount_cent),amount_cent,NULL)`, item.ID, item.OrderNo, item.AmountCent)
	return err
}

func (h *Handler) settlePayment(ctx context.Context, result wechat.PayOrder) error {
	if result.AppID != h.appID || result.MerchantID != h.merchantID || result.OrderNo == "" || result.TransactionID == "" || result.TradeState != "SUCCESS" || result.Currency != "CNY" || result.AmountCent <= 0 || result.SuccessTime.IsZero() {
		return errPaymentIncomplete
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderID, userID, amountCent uint64
	var status string
	var snapshotJSON []byte
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,status,total_amount_cent,product_snapshot FROM orders WHERE order_no=? FOR UPDATE`, result.OrderNo).
		Scan(&orderID, &userID, &status, &amountCent, &snapshotJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return errOrderNotFound
	}
	if err != nil {
		return err
	}
	var paymentID, paymentAmount uint64
	var paymentStatus string
	var existingTransactionID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,status,amount_cent,provider_transaction_id FROM payment_transactions WHERE order_id=? AND merchant_order_no=? FOR UPDATE`, orderID, result.OrderNo).
		Scan(&paymentID, &paymentStatus, &paymentAmount, &existingTransactionID)
	if err != nil {
		return err
	}
	if amountCent != uint64(result.AmountCent) || paymentAmount != amountCent || (existingTransactionID.Valid && existingTransactionID.String != result.TransactionID) {
		return errPaymentMismatch
	}
	if status == "PAID" && paymentStatus == "SUCCEEDED" {
		var cardCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM member_cards WHERE source_order_id=?`, orderID).Scan(&cardCount); err != nil || cardCount != 1 {
			return errPaymentMismatch
		}
		return tx.Commit()
	}
	if status != "PENDING" && status != "CLOSED" {
		return errOrderNotPayable
	}
	var snapshot order.ProductSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return err
	}
	productID, err := strconv.ParseUint(snapshot.ProductID, 10, 64)
	if err != nil || productID == 0 {
		return errPaymentMismatch
	}
	cardNo, err := randomBusinessNo("URCARD")
	if err != nil {
		return err
	}
	cardStatus := "PENDING_ACTIVATION"
	var activatedAt, expiresAt any
	if snapshot.ActivationMode == "PURCHASE" {
		cardStatus = "ACTIVE"
		activatedAt = result.SuccessTime.UTC()
		expiresAt = result.SuccessTime.UTC().AddDate(0, 0, int(snapshot.ValidityDays))
	}
	var totalTimes, remainingTimes any
	if snapshot.ProductType == "COUNT_CARD" {
		if snapshot.TotalTimes == nil || *snapshot.TotalTimes == 0 {
			return errPaymentMismatch
		}
		totalTimes, remainingTimes = *snapshot.TotalTimes, *snapshot.TotalTimes
	} else if snapshot.ProductType != "TIME_PASS" {
		return errPaymentMismatch
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO member_cards(card_no,user_id,source_order_id,product_id,product_name,product_type,total_times,remaining_times,status,activated_at,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, cardNo, userID, orderID, productID, snapshot.Name, snapshot.ProductType, totalTimes, remainingTimes, cardStatus, activatedAt, expiresAt)
	if err != nil {
		return err
	}
	auditPayload, _ := json.Marshal(map[string]any{"trade_state": "SUCCESS", "transaction_id": result.TransactionID, "success_time": result.SuccessTime.UTC().Format(time.RFC3339)})
	if _, err := tx.ExecContext(ctx, `UPDATE payment_transactions SET status='SUCCEEDED',provider_transaction_id=?,callback_payload=?,succeeded_at=? WHERE id=?`, result.TransactionID, auditPayload, result.SuccessTime.UTC(), paymentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='PAID',paid_amount_cent=?,paid_at=?,closed_at=NULL WHERE id=?`, amountCent, result.SuccessTime.UTC(), orderID); err != nil {
		return err
	}
	return tx.Commit()
}

func randomBusinessNo(prefix string) (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s%s", prefix, time.Now().UTC().Format("20060102150405"), strings.ToUpper(hex.EncodeToString(value))), nil
}

func writeNotifyResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
