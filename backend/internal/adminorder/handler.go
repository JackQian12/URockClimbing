package adminorder

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db          *sql.DB
	auth        *adminauth.Handler
	phoneCipher *securefield.Cipher
}

type Order struct {
	ID                     string          `json:"id"`
	OrderNo                string          `json:"order_no"`
	Status                 string          `json:"status"`
	BizType                string          `json:"biz_type"`
	TotalAmountCent        uint64          `json:"total_amount_cent"`
	PaidAmountCent         uint64          `json:"paid_amount_cent"`
	ProductSnapshot        json.RawMessage `json:"product_snapshot"`
	MemberID               string          `json:"member_id"`
	MemberNo               string          `json:"member_no"`
	MemberNickname         *string         `json:"member_nickname"`
	MemberPhone            *string         `json:"member_phone"`
	PaymentStatus          *string         `json:"payment_status"`
	MerchantOrderNo        *string         `json:"merchant_order_no"`
	ProviderTransactionID  *string         `json:"provider_transaction_id"`
	CardNo                 *string         `json:"card_no"`
	CardStatus             *string         `json:"card_status"`
	CardTotalTimes         *uint           `json:"card_total_times"`
	CardRemainingTimes     *uint           `json:"card_remaining_times"`
	CardExpiresAt          *string         `json:"card_expires_at"`
	RedemptionCount        uint            `json:"redemption_count"`
	RefundNo               *string         `json:"refund_no"`
	RefundStatus           *string         `json:"refund_status"`
	RefundReason           *string         `json:"refund_reason"`
	RefundCreatedAt        *string         `json:"refund_created_at"`
	RefundEligible         bool            `json:"refund_eligible"`
	RefundIneligibleReason string          `json:"refund_ineligible_reason"`
	ExpiresAt              string          `json:"expires_at"`
	PaidAt                 *string         `json:"paid_at"`
	ClosedAt               *string         `json:"closed_at"`
	CreatedAt              string          `json:"created_at"`
	UpdatedAt              string          `json:"updated_at"`
}

func NewHandler(db *sql.DB, auth *adminauth.Handler, phoneCipher *securefield.Cipher) *Handler {
	return &Handler{db: db, auth: auth, phoneCipher: phoneCipher}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validOrderStatus(status) {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_ORDER_STATUS", "订单状态筛选无效")
		return
	}
	page := boundedInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := boundedInt(r.URL.Query().Get("page_size"), 20, 1, 50)
	where, args := orderWhere(query, status)
	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders o JOIN users u ON u.id=o.user_id LEFT JOIN member_profiles mp ON mp.user_id=u.id `+where, args...).Scan(&total); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "订单列表加载失败")
		return
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(), orderSelect+where+` ORDER BY o.created_at DESC, o.id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单列表加载失败")
		return
	}
	defer rows.Close()
	items := make([]Order, 0)
	for rows.Next() {
		item, scanErr := scanOrder(rows, h.phoneCipher)
		if scanErr != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "订单列表加载失败")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单列表加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	orderNo := strings.TrimSpace(r.PathValue("order_no"))
	if orderNo == "" || len(orderNo) > 64 {
		respond.Error(w, r, 400, "INVALID_ORDER_NO", "订单号无效")
		return
	}
	item, err := h.get(r.Context(), orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, 404, "ORDER_NOT_FOUND", "订单不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单详情加载失败")
		return
	}
	respond.JSON(w, r, 200, item)
}

// Refund intentionally refuses to alter order state until the WeChat Pay adapter is configured.
// This prevents a database-only "refund" from being mistaken for a real funds movement.
func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, true); !ok {
		return
	}
	var input struct {
		Reason          string `json:"reason"`
		ClientRequestID string `json:"client_request_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Reason) == "" || len([]rune(strings.TrimSpace(input.Reason))) > 255 || strings.TrimSpace(input.ClientRequestID) == "" {
		respond.Error(w, r, 422, "INVALID_REFUND_REQUEST", "请填写退款原因并重新提交")
		return
	}
	orderNo := strings.TrimSpace(r.PathValue("order_no"))
	item, err := h.get(r.Context(), orderNo)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, 404, "ORDER_NOT_FOUND", "订单不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单详情加载失败")
		return
	}
	if !item.RefundEligible {
		respond.Error(w, r, 409, "REFUND_NOT_ALLOWED", item.RefundIneligibleReason)
		return
	}
	respond.Error(w, r, http.StatusServiceUnavailable, "WECHAT_REFUND_NOT_CONFIGURED", "微信支付退款尚未配置，订单未发生任何变更")
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, mutation bool) (adminauth.Session, bool) {
	session, ok := h.auth.Authenticate(r)
	if !ok {
		respond.Error(w, r, 401, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return adminauth.Session{}, false
	}
	if session.MustChangePassword {
		respond.Error(w, r, 403, "PASSWORD_CHANGE_REQUIRED", "请先修改初始密码")
		return adminauth.Session{}, false
	}
	if mutation && !h.auth.ValidCSRF(session, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, 403, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return adminauth.Session{}, false
	}
	return session, true
}

func validOrderStatus(value string) bool {
	switch value {
	case "PENDING", "PAID", "CLOSED", "REFUNDING", "REFUNDED":
		return true
	}
	return false
}

func orderWhere(query, status string) (string, []any) {
	where := ` WHERE 1=1`
	args := make([]any, 0, 5)
	if query != "" {
		like := "%" + escapeLike(query) + "%"
		where += ` AND (o.order_no LIKE ? ESCAPE '!' OR u.member_no LIKE ? ESCAPE '!' OR u.nickname LIKE ? ESCAPE '!' OR mp.phone_last4=?)`
		args = append(args, like, like, like, query)
	}
	if status != "" {
		where += ` AND o.status=?`
		args = append(args, status)
	}
	return where, args
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	return strings.ReplaceAll(value, "_", "!_")
}

const orderSelect = `
SELECT o.id,o.order_no,o.status,o.biz_type,o.total_amount_cent,o.paid_amount_cent,o.product_snapshot,
       u.id,u.member_no,u.nickname,u.phone_encrypted,
       pt.status,pt.merchant_order_no,pt.provider_transaction_id,
       mc.card_no,mc.status,mc.total_times,mc.remaining_times,mc.expires_at,
       (SELECT COUNT(*) FROM redemption_records rr WHERE rr.member_card_id=mc.id),
       rt.refund_no,rt.status,rt.reason,rt.created_at,
       o.expires_at,o.paid_at,o.closed_at,o.created_at,o.updated_at
FROM orders o
JOIN users u ON u.id=o.user_id
LEFT JOIN member_profiles mp ON mp.user_id=u.id
LEFT JOIN payment_transactions pt ON pt.id=(SELECT MAX(pt2.id) FROM payment_transactions pt2 WHERE pt2.order_id=o.id)
LEFT JOIN member_cards mc ON mc.source_order_id=o.id
LEFT JOIN refund_transactions rt ON rt.order_id=o.id`

type scanner interface{ Scan(...any) error }

func scanOrder(row scanner, phoneCipher *securefield.Cipher) (Order, error) {
	var item Order
	var id, memberID uint64
	var snapshot []byte
	var nickname, payStatus, merchantNo, transactionID sql.NullString
	var cardNo, cardStatus, refundNo, refundStatus, refundReason sql.NullString
	var phoneEncrypted []byte
	var cardTotal, cardRemaining sql.NullInt64
	var cardExpires, refundCreated, paidAt, closedAt sql.NullTime
	var expiresAt, createdAt, updatedAt time.Time
	err := row.Scan(&id, &item.OrderNo, &item.Status, &item.BizType, &item.TotalAmountCent, &item.PaidAmountCent, &snapshot,
		&memberID, &item.MemberNo, &nickname, &phoneEncrypted, &payStatus, &merchantNo, &transactionID,
		&cardNo, &cardStatus, &cardTotal, &cardRemaining, &cardExpires, &item.RedemptionCount,
		&refundNo, &refundStatus, &refundReason, &refundCreated, &expiresAt, &paidAt, &closedAt, &createdAt, &updatedAt)
	if err != nil {
		return Order{}, err
	}
	item.ID = strconv.FormatUint(id, 10)
	item.MemberID = strconv.FormatUint(memberID, 10)
	item.ProductSnapshot = json.RawMessage(snapshot)
	item.MemberNickname = nullString(nickname)
	item.PaymentStatus = nullString(payStatus)
	item.MerchantOrderNo = nullString(merchantNo)
	item.ProviderTransactionID = nullString(transactionID)
	item.CardNo = nullString(cardNo)
	item.CardStatus = nullString(cardStatus)
	item.RefundNo = nullString(refundNo)
	item.RefundStatus = nullString(refundStatus)
	item.RefundReason = nullString(refundReason)
	if cardTotal.Valid {
		v := uint(cardTotal.Int64)
		item.CardTotalTimes = &v
	}
	if cardRemaining.Valid {
		v := uint(cardRemaining.Int64)
		item.CardRemainingTimes = &v
	}
	item.CardExpiresAt = timeString(cardExpires)
	item.RefundCreatedAt = timeString(refundCreated)
	item.PaidAt = timeString(paidAt)
	item.ClosedAt = timeString(closedAt)
	item.ExpiresAt = formatTime(expiresAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	if len(phoneEncrypted) > 0 {
		if phoneCipher == nil {
			return Order{}, errors.New("phone cipher is not configured")
		}
		phone, err := phoneCipher.Decrypt(phoneEncrypted)
		if err != nil {
			return Order{}, err
		}
		item.MemberPhone = &phone
	}
	item.RefundEligible, item.RefundIneligibleReason = refundEligibility(item, time.Now().UTC())
	return item, nil
}

func refundEligibility(item Order, now time.Time) (bool, string) {
	if item.RefundStatus != nil {
		return false, "该订单已经存在退款记录"
	}
	if item.Status != "PAID" {
		return false, "只有已支付订单可以退款"
	}
	if item.PaymentStatus == nil || *item.PaymentStatus != "SUCCEEDED" {
		return false, "未确认微信支付成功"
	}
	if item.CardNo == nil {
		return false, "订单尚未发放次卡"
	}
	if item.RedemptionCount > 0 {
		return false, "次卡已有核销记录，不能自动退款"
	}
	if item.CardStatus == nil || *item.CardStatus != "ACTIVE" {
		return false, "次卡当前状态不允许退款"
	}
	if item.PaidAt == nil {
		return false, "缺少支付成功时间"
	}
	paidAt, err := time.Parse(time.RFC3339Nano, *item.PaidAt)
	if err != nil || now.Sub(paidAt) > 365*24*time.Hour {
		return false, "订单已超过微信支付退款期限"
	}
	return true, ""
}

func (h *Handler) get(ctx context.Context, orderNo string) (Order, error) {
	return scanOrder(h.db.QueryRowContext(ctx, orderSelect+` WHERE o.order_no=?`, orderNo), h.phoneCipher)
}
func nullString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func timeString(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	s := formatTime(v.Time)
	return &s
}
func formatTime(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
func boundedInt(raw string, fallback, min, max int) int {
	v, e := strconv.Atoi(raw)
	if e != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
