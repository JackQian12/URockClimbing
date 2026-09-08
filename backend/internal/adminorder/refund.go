package adminorder

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/platform/wechat"
	"urockclimbing.com/backend/internal/respond"
)

var (
	errRefundNotFound   = errors.New("refund not found")
	errRefundNotAllowed = errors.New("refund not allowed")
	errRefundExists     = errors.New("refund already exists")
	errRefundMismatch   = errors.New("refund does not match local transaction")
)

type RefundRecord struct {
	RefundNo         string  `json:"refund_no"`
	OrderNo          string  `json:"order_no"`
	ProviderRefundID *string `json:"provider_refund_id"`
	Status           string  `json:"status"`
	AmountCent       uint64  `json:"amount_cent"`
	Reason           string  `json:"reason"`
	ClientRequestID  string  `json:"client_request_id"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if h.refunds == nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "WECHAT_REFUND_NOT_CONFIGURED", "微信支付退款配置尚未完成，订单未发生任何变更")
		return
	}
	var input struct {
		Reason          string `json:"reason"`
		ClientRequestID string `json:"client_request_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond.Error(w, r, 422, "INVALID_REFUND_REQUEST", "请填写退款原因并重新提交")
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)
	if input.Reason == "" || len([]byte(input.Reason)) > 80 || len(input.ClientRequestID) < 8 || len(input.ClientRequestID) > 64 {
		respond.Error(w, r, 422, "INVALID_REFUND_REQUEST", "退款原因不得超过 80 字节，请重新提交")
		return
	}
	orderNo := strings.TrimSpace(r.PathValue("order_no"))
	record, existing, transactionID, err := h.lockForRefund(r.Context(), session, orderNo, input.Reason, input.ClientRequestID, r)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respond.Error(w, r, 404, "ORDER_NOT_FOUND", "订单不存在")
		return
	case errors.Is(err, errRefundExists):
		respond.Error(w, r, 409, "REFUND_ALREADY_EXISTS", "该订单已有退款记录")
		return
	case errors.Is(err, errRefundNotAllowed):
		respond.Error(w, r, 409, "REFUND_NOT_ALLOWED", "仅支持未核销、未退款的已支付订单原路全额退款")
		return
	case err != nil:
		respond.Error(w, r, 500, "REFUND_CREATE_FAILED", "退款单创建失败")
		return
	}
	if existing && record.Status != "CREATED" {
		respond.JSON(w, r, http.StatusOK, record)
		return
	}
	result, err := h.refunds.CreateRefund(r.Context(), wechat.RefundInput{
		RefundNo: record.RefundNo, OrderNo: record.OrderNo, TransactionID: transactionID,
		Reason: record.Reason, AmountCent: int64(record.AmountCent),
	})
	if err != nil {
		var rejected *wechat.RefundRejectedError
		if errors.As(err, &rejected) {
			if failErr := h.failRefundRequest(r.Context(), record.RefundNo, rejected.Code, middleware.RequestIDFromContext(r.Context())); failErr != nil {
				respond.Error(w, r, 500, "REFUND_FAILURE_SYNC_FAILED", "微信已拒绝退款，本地状态恢复失败，请联系技术支持")
				return
			}
			respond.Error(w, r, 409, "WECHAT_REFUND_REJECTED", "微信支付未受理该退款，会员卡已恢复")
			return
		}
		// The remote outcome is unknown. Keep the card locked until a signed query
		// or callback gives a final result, and reuse the same refund number on retry.
		respond.Error(w, r, http.StatusBadGateway, "WECHAT_REFUND_UNKNOWN", "退款结果暂时未知，会员卡已锁定，请稍后同步退款状态")
		return
	}
	if err := h.applyRefundResult(r.Context(), result, middleware.RequestIDFromContext(r.Context())); err != nil {
		respond.Error(w, r, 500, "REFUND_SYNC_FAILED", "退款已受理，本地状态同步失败，请稍后重试")
		return
	}
	record, err = h.getRefund(r.Context(), record.RefundNo)
	if err != nil {
		respond.Error(w, r, 500, "REFUND_READ_FAILED", "退款状态读取失败")
		return
	}
	status := http.StatusAccepted
	if existing || record.Status == "SUCCEEDED" || record.Status == "FAILED" {
		status = http.StatusOK
	}
	respond.JSON(w, r, status, record)
}

func (h *Handler) failRefundRequest(ctx context.Context, refundNo, rejectionCode, requestID string) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var refundID, orderID, cardID, operatorID uint64
	var localStatus string
	var activatedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT rt.id,rt.status,rt.operator_user_id,o.id,mc.id,mc.activated_at
		FROM refund_transactions rt JOIN orders o ON o.id=rt.order_id JOIN member_cards mc ON mc.source_order_id=o.id
		WHERE rt.refund_no=? FOR UPDATE`, refundNo).
		Scan(&refundID, &localStatus, &operatorID, &orderID, &cardID, &activatedAt)
	if err != nil {
		return err
	}
	if localStatus == "FAILED" {
		return tx.Commit()
	}
	if localStatus != "CREATED" {
		return errRefundMismatch
	}
	payload, _ := json.Marshal(map[string]string{"rejection_code": rejectionCode})
	if _, err := tx.ExecContext(ctx, `UPDATE refund_transactions SET status='FAILED',callback_payload=?,failed_at=UTC_TIMESTAMP(3) WHERE id=?`, payload, refundID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='PAID' WHERE id=? AND status='REFUNDING'`, orderID); err != nil {
		return err
	}
	restoredStatus := "ACTIVE"
	if !activatedAt.Valid {
		restoredStatus = "PENDING_ACTIVATION"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE member_cards SET status=? WHERE id=? AND status='REFUND_LOCKED'`, restoredStatus, cardID); err != nil {
		return err
	}
	after, _ := json.Marshal(map[string]string{"refund_status": "FAILED", "rejection_code": rejectionCode})
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,after_snapshot,request_id) VALUES(?,'ORDER_REFUND_REJECTED','REFUND',?,?,?)`, operatorID, refundNo, after, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) SyncRefund(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, true); !ok {
		return
	}
	if h.refunds == nil {
		respond.Error(w, r, 503, "WECHAT_REFUND_NOT_CONFIGURED", "微信支付退款配置尚未完成")
		return
	}
	refundNo := strings.TrimSpace(r.PathValue("refund_no"))
	if _, err := h.getRefund(r.Context(), refundNo); errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, 404, "REFUND_NOT_FOUND", "退款单不存在")
		return
	} else if err != nil {
		respond.Error(w, r, 500, "REFUND_READ_FAILED", "退款单读取失败")
		return
	}
	result, err := h.refunds.QueryRefund(r.Context(), refundNo)
	if err != nil {
		respond.Error(w, r, 502, "WECHAT_REFUND_QUERY_FAILED", "退款状态查询失败")
		return
	}
	if err := h.applyRefundResult(r.Context(), result, middleware.RequestIDFromContext(r.Context())); err != nil {
		respond.Error(w, r, 409, "REFUND_SYNC_FAILED", "退款结果校验失败，请联系技术支持")
		return
	}
	record, _ := h.getRefund(r.Context(), refundNo)
	respond.JSON(w, r, 200, record)
}

func (h *Handler) RefundNotify(w http.ResponseWriter, r *http.Request) {
	if h.refunds == nil {
		writeRefundNotifyResponse(w, 503, "FAIL", "refund is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	result, err := h.refunds.ParseRefundNotification(r.Context(), r)
	if err != nil {
		writeRefundNotifyResponse(w, 400, "FAIL", "invalid notification")
		return
	}
	if err := h.applyRefundResult(r.Context(), result, middleware.RequestIDFromContext(r.Context())); err != nil {
		writeRefundNotifyResponse(w, 500, "FAIL", "refund was not synchronized")
		return
	}
	writeRefundNotifyResponse(w, 200, "SUCCESS", "成功")
}

func (h *Handler) lockForRefund(ctx context.Context, session adminauth.Session, orderNo, reason, clientRequestID string, request *http.Request) (RefundRecord, bool, string, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return RefundRecord{}, false, "", err
	}
	defer tx.Rollback()
	var orderID, paymentID, cardID, paidAmount uint64
	var orderStatus, paymentStatus, transactionID, cardStatus string
	var paidAt time.Time
	var activatedAt sql.NullTime
	var redemptionCount int
	err = tx.QueryRowContext(ctx, `
		SELECT o.id,o.status,o.paid_amount_cent,o.paid_at,pt.id,pt.status,pt.provider_transaction_id,mc.id,mc.status,mc.activated_at,
		       (SELECT COUNT(*) FROM redemption_records rr WHERE rr.member_card_id=mc.id)
		FROM orders o
		JOIN payment_transactions pt ON pt.order_id=o.id AND pt.status='SUCCEEDED'
		JOIN member_cards mc ON mc.source_order_id=o.id
		WHERE o.order_no=? FOR UPDATE`, orderNo).
		Scan(&orderID, &orderStatus, &paidAmount, &paidAt, &paymentID, &paymentStatus, &transactionID, &cardID, &cardStatus, &activatedAt, &redemptionCount)
	if err != nil {
		return RefundRecord{}, false, "", err
	}
	var existing RefundRecord
	var existingOperator uint64
	var existingProviderID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT rt.refund_no,o.order_no,rt.provider_refund_id,rt.status,rt.amount_cent,rt.reason,rt.client_request_id,rt.operator_user_id FROM refund_transactions rt JOIN orders o ON o.id=rt.order_id WHERE rt.order_id=? FOR UPDATE`, orderID).
		Scan(&existing.RefundNo, &existing.OrderNo, &existingProviderID, &existing.Status, &existing.AmountCent, &existing.Reason, &existing.ClientRequestID, &existingOperator)
	if err == nil {
		existing.ProviderRefundID = nullString(existingProviderID)
		if existingOperator == session.UserID && existing.ClientRequestID == clientRequestID {
			if err := tx.Commit(); err != nil {
				return RefundRecord{}, false, "", err
			}
			fresh, err := h.getRefund(ctx, existing.RefundNo)
			return fresh, true, transactionID, err
		}
		return RefundRecord{}, false, "", errRefundExists
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RefundRecord{}, false, "", err
	}
	if orderStatus != "PAID" || paymentStatus != "SUCCEEDED" || paidAmount == 0 || redemptionCount != 0 || (cardStatus != "ACTIVE" && cardStatus != "PENDING_ACTIVATION") || time.Since(paidAt.UTC()) > 365*24*time.Hour {
		return RefundRecord{}, false, "", errRefundNotAllowed
	}
	refundNo, err := refundBusinessNo()
	if err != nil {
		return RefundRecord{}, false, "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO refund_transactions(refund_no,order_id,payment_transaction_id,status,amount_cent,reason,operator_user_id,client_request_id) VALUES(?,?,?,'CREATED',?,?,?,?)`, refundNo, orderID, paymentID, paidAmount, reason, session.UserID, clientRequestID)
	if err != nil {
		return RefundRecord{}, false, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='REFUNDING' WHERE id=? AND status='PAID'`, orderID); err != nil {
		return RefundRecord{}, false, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE member_cards SET status='REFUND_LOCKED' WHERE id=? AND status IN ('ACTIVE','PENDING_ACTIVATION')`, cardID); err != nil {
		return RefundRecord{}, false, "", err
	}
	before, _ := json.Marshal(map[string]any{"order_status": "PAID", "card_status": cardStatus})
	after, _ := json.Marshal(map[string]any{"order_status": "REFUNDING", "card_status": "REFUND_LOCKED", "refund_no": refundNo})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,before_snapshot,after_snapshot,request_ip,request_id) VALUES(?,'ORDER_REFUND_REQUEST','ORDER',?,?,?,?,?)`, session.UserID, orderNo, before, after, strings.TrimSpace(request.Header.Get("X-Real-IP")), middleware.RequestIDFromContext(ctx))
	if err != nil {
		return RefundRecord{}, false, "", err
	}
	if err := tx.Commit(); err != nil {
		return RefundRecord{}, false, "", err
	}
	record, err := h.getRefund(ctx, refundNo)
	return record, false, transactionID, err
}

func (h *Handler) applyRefundResult(ctx context.Context, result wechat.RefundResult, requestID string) error {
	if result.MerchantID != h.merchantID || result.RefundNo == "" || result.OrderNo == "" || result.TransactionID == "" || result.ProviderRefundID == "" || result.Currency != "CNY" || result.TotalCent <= 0 || result.RefundCent != result.TotalCent {
		return errRefundMismatch
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var refundID, orderID, cardID, operatorID, amountCent uint64
	var localStatus, orderNo, transactionID, orderStatus, cardStatus string
	var providerRefundID sql.NullString
	var activatedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT rt.id,rt.status,rt.provider_refund_id,rt.amount_cent,rt.operator_user_id,o.id,o.order_no,o.status,pt.provider_transaction_id,mc.id,mc.status,mc.activated_at
		FROM refund_transactions rt JOIN orders o ON o.id=rt.order_id JOIN payment_transactions pt ON pt.id=rt.payment_transaction_id JOIN member_cards mc ON mc.source_order_id=o.id
		WHERE rt.refund_no=? FOR UPDATE`, result.RefundNo).
		Scan(&refundID, &localStatus, &providerRefundID, &amountCent, &operatorID, &orderID, &orderNo, &orderStatus, &transactionID, &cardID, &cardStatus, &activatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return errRefundNotFound
	}
	if err != nil {
		return err
	}
	if orderNo != result.OrderNo || transactionID != result.TransactionID || amountCent != uint64(result.RefundCent) || (providerRefundID.Valid && providerRefundID.String != result.ProviderRefundID) {
		return errRefundMismatch
	}
	if localStatus == "SUCCEEDED" || localStatus == "FAILED" {
		return tx.Commit()
	}
	auditPayload, _ := json.Marshal(map[string]any{"refund_status": result.Status, "refund_id": result.ProviderRefundID})
	switch result.Status {
	case "SUCCESS":
		if result.SuccessTime.IsZero() {
			return errRefundMismatch
		}
		if _, err := tx.ExecContext(ctx, `UPDATE refund_transactions SET status='SUCCEEDED',provider_refund_id=?,callback_payload=?,succeeded_at=? WHERE id=?`, result.ProviderRefundID, auditPayload, result.SuccessTime.UTC(), refundID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='REFUNDED' WHERE id=? AND status IN ('REFUNDING','REFUNDED')`, orderID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE member_cards SET status='REFUNDED' WHERE id=? AND status IN ('REFUND_LOCKED','REFUNDED')`, cardID); err != nil {
			return err
		}
	case "CLOSED":
		if _, err := tx.ExecContext(ctx, `UPDATE refund_transactions SET status='FAILED',provider_refund_id=?,callback_payload=?,failed_at=UTC_TIMESTAMP(3) WHERE id=?`, result.ProviderRefundID, auditPayload, refundID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status='PAID' WHERE id=? AND status='REFUNDING'`, orderID); err != nil {
			return err
		}
		restoredStatus := "ACTIVE"
		if !activatedAt.Valid {
			restoredStatus = "PENDING_ACTIVATION"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE member_cards SET status=? WHERE id=? AND status='REFUND_LOCKED'`, restoredStatus, cardID); err != nil {
			return err
		}
	case "PROCESSING":
		_, err = tx.ExecContext(ctx, `UPDATE refund_transactions SET status='PROCESSING',provider_refund_id=?,callback_payload=? WHERE id=? AND status IN ('CREATED','PROCESSING')`, result.ProviderRefundID, auditPayload, refundID)
		if err != nil {
			return err
		}
	case "ABNORMAL":
		_, err = tx.ExecContext(ctx, `UPDATE refund_transactions SET status='ABNORMAL',provider_refund_id=?,callback_payload=? WHERE id=? AND status IN ('CREATED','PROCESSING','ABNORMAL')`, result.ProviderRefundID, auditPayload, refundID)
		if err != nil {
			return err
		}
	default:
		return errRefundMismatch
	}
	before, _ := json.Marshal(map[string]any{"refund_status": localStatus, "order_status": orderStatus, "card_status": cardStatus})
	after, _ := json.Marshal(map[string]any{"refund_status": result.Status})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,before_snapshot,after_snapshot,request_id) VALUES(?,'ORDER_REFUND_SYNC','REFUND',?,?,?,?)`, operatorID, result.RefundNo, before, after, requestID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) getRefund(ctx context.Context, refundNo string) (RefundRecord, error) {
	var item RefundRecord
	var providerRefundID sql.NullString
	var createdAt, updatedAt time.Time
	err := h.db.QueryRowContext(ctx, `SELECT rt.refund_no,o.order_no,rt.provider_refund_id,rt.status,rt.amount_cent,rt.reason,rt.client_request_id,rt.created_at,rt.updated_at FROM refund_transactions rt JOIN orders o ON o.id=rt.order_id WHERE rt.refund_no=?`, refundNo).
		Scan(&item.RefundNo, &item.OrderNo, &providerRefundID, &item.Status, &item.AmountCent, &item.Reason, &item.ClientRequestID, &createdAt, &updatedAt)
	if err != nil {
		return RefundRecord{}, err
	}
	item.ProviderRefundID = nullString(providerRefundID)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func refundBusinessNo() (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "URREF" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(value)), nil
}

func writeRefundNotifyResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
