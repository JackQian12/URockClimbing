// Package order owns order creation, snapshots, and member order queries.
package order

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/respond"
)

const orderTTL = 30 * time.Minute

var (
	errProductNotFound = errors.New("card product is not on sale")
	errPurchaseLimit   = errors.New("card product purchase limit reached")
)

type Handler struct {
	db   *sql.DB
	auth *auth.Handler
	now  func() time.Time
}

type ProductSnapshot struct {
	ProductID      string `json:"product_id"`
	Name           string `json:"name"`
	ProductType    string `json:"product_type"`
	TotalTimes     *uint  `json:"total_times"`
	ValidityDays   uint   `json:"validity_days"`
	ActivationMode string `json:"activation_mode"`
	PriceCent      uint64 `json:"price_cent"`
	DailyUseLimit  uint   `json:"daily_use_limit"`
	Transferable   bool   `json:"transferable"`
}

type Order struct {
	OrderNo         string          `json:"order_no"`
	Status          string          `json:"status"`
	BizType         string          `json:"biz_type"`
	TotalAmountCent uint64          `json:"total_amount_cent"`
	PaidAmountCent  uint64          `json:"paid_amount_cent"`
	ProductSnapshot ProductSnapshot `json:"product_snapshot"`
	ExpiresAt       string          `json:"expires_at"`
	PaidAt          *string         `json:"paid_at"`
	ClosedAt        *string         `json:"closed_at"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

func NewHandler(db *sql.DB, authenticator *auth.Handler) *Handler {
	return &Handler{db: db, auth: authenticator, now: time.Now}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, err := h.registeredUser(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	var input struct {
		CardProductID string `json:"card_product_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if decoder.Decode(&input) != nil || len(idempotencyKey) < 8 || len(idempotencyKey) > 64 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_ORDER_REQUEST", "购卡请求无效，请重新提交")
		return
	}
	productID, err := strconv.ParseUint(strings.TrimSpace(input.CardProductID), 10, 64)
	if err != nil || productID == 0 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_CARD_PRODUCT", "会员卡商品无效")
		return
	}
	item, created, err := h.create(r.Context(), user.ID, productID, idempotencyKey)
	switch {
	case errors.Is(err, errProductNotFound):
		respond.Error(w, r, http.StatusConflict, "CARD_PRODUCT_NOT_ON_SALE", "该会员卡当前不可购买")
	case errors.Is(err, errPurchaseLimit):
		respond.Error(w, r, http.StatusConflict, "PURCHASE_LIMIT_REACHED", "该会员卡已达到限购数量")
	case err != nil:
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "订单创建失败，请稍后重试")
	default:
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		respond.JSON(w, r, status, item)
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, err := h.registeredUser(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.closeExpired(r.Context(), user.ID)
	page := boundedInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := boundedInt(r.URL.Query().Get("page_size"), 20, 1, 100)
	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders WHERE user_id=?`, user.ID).Scan(&total); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单加载失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), orderSelect+` WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, user.ID, pageSize, (page-1)*pageSize)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单加载失败")
		return
	}
	defer rows.Close()
	items := make([]Order, 0)
	for rows.Next() {
		item, scanErr := scanOrder(rows)
		if scanErr != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "订单加载失败")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "订单加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.registeredUser(r)
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.closeExpired(r.Context(), user.ID)
	item, err := h.get(r.Context(), user.ID, strings.TrimSpace(r.PathValue("order_no")))
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

func (h *Handler) registeredUser(r *http.Request) (auth.CurrentUser, error) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		return auth.CurrentUser{}, err
	}
	if !user.Registered {
		return auth.CurrentUser{}, errRegistrationRequired
	}
	return user, nil
}

var errRegistrationRequired = errors.New("registration required")

func (h *Handler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRegistrationRequired) {
		respond.Error(w, r, 403, "REGISTRATION_REQUIRED", "请授权手机号完成会员注册")
		return
	}
	respond.Error(w, r, 401, "UNAUTHENTICATED", "请先登录")
}

func (h *Handler) create(ctx context.Context, userID, productID uint64, clientRequestID string) (Order, bool, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, false, err
	}
	defer tx.Rollback()
	if item, err := getOrder(ctx, tx, userID, "", clientRequestID); err == nil {
		return item, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, err
	}

	var name, productType, activationMode string
	var totalTimes, purchaseLimit sql.NullInt64
	var validityDays, dailyUseLimit uint
	var priceCent uint64
	var transferable bool
	err = tx.QueryRowContext(ctx, `SELECT name,product_type,total_times,validity_days,activation_mode,price_cent,purchase_limit,daily_use_limit,transferable FROM card_products WHERE id=? AND status='ON_SALE' FOR SHARE`, productID).
		Scan(&name, &productType, &totalTimes, &validityDays, &activationMode, &priceCent, &purchaseLimit, &dailyUseLimit, &transferable)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, errProductNotFound
	}
	if err != nil {
		return Order{}, false, err
	}
	if purchaseLimit.Valid {
		var purchased int64
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE user_id=? AND JSON_UNQUOTE(JSON_EXTRACT(product_snapshot,'$.product_id'))=? AND status IN ('PENDING','PAID','REFUNDING')`, userID, strconv.FormatUint(productID, 10)).Scan(&purchased)
		if err != nil {
			return Order{}, false, err
		}
		if purchased >= purchaseLimit.Int64 {
			return Order{}, false, errPurchaseLimit
		}
	}
	snapshot := ProductSnapshot{ProductID: strconv.FormatUint(productID, 10), Name: name, ProductType: productType, ValidityDays: validityDays, ActivationMode: activationMode, PriceCent: priceCent, DailyUseLimit: dailyUseLimit, Transferable: transferable}
	if totalTimes.Valid {
		value := uint(totalTimes.Int64)
		snapshot.TotalTimes = &value
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return Order{}, false, err
	}
	orderNo, err := randomBusinessNo("URORD")
	if err != nil {
		return Order{}, false, err
	}
	now := h.now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO orders(order_no,user_id,biz_type,status,total_amount_cent,paid_amount_cent,product_snapshot,client_request_id,expires_at) VALUES(?,?,'CARD_PURCHASE','PENDING',?,0,?,?,?)`, orderNo, userID, priceCent, snapshotJSON, clientRequestID, now.Add(orderTTL))
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			item, getErr := getOrder(ctx, h.db, userID, "", clientRequestID)
			return item, false, getErr
		}
		return Order{}, false, err
	}
	item, err := getOrder(ctx, tx, userID, orderNo, "")
	if err != nil {
		return Order{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, false, err
	}
	return item, true, nil
}

func (h *Handler) closeExpired(ctx context.Context, userID uint64) {
	_, _ = h.db.ExecContext(ctx, `UPDATE orders SET status='CLOSED',closed_at=UTC_TIMESTAMP(3) WHERE user_id=? AND status='PENDING' AND expires_at<=UTC_TIMESTAMP(3)`, userID)
}

func (h *Handler) get(ctx context.Context, userID uint64, orderNo string) (Order, error) {
	if orderNo == "" || len(orderNo) > 64 {
		return Order{}, sql.ErrNoRows
	}
	return getOrder(ctx, h.db, userID, orderNo, "")
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getOrder(ctx context.Context, q queryer, userID uint64, orderNo, clientRequestID string) (Order, error) {
	query := orderSelect + ` WHERE user_id=? AND order_no=?`
	args := []any{userID, orderNo}
	if clientRequestID != "" {
		query = orderSelect + ` WHERE user_id=? AND client_request_id=?`
		args = []any{userID, clientRequestID}
	}
	return scanOrder(q.QueryRowContext(ctx, query, args...))
}

const orderSelect = `SELECT order_no,status,biz_type,total_amount_cent,paid_amount_cent,product_snapshot,expires_at,paid_at,closed_at,created_at,updated_at FROM orders`

type scanner interface{ Scan(...any) error }

func scanOrder(row scanner) (Order, error) {
	var item Order
	var snapshot []byte
	var expiresAt, createdAt, updatedAt time.Time
	var paidAt, closedAt sql.NullTime
	err := row.Scan(&item.OrderNo, &item.Status, &item.BizType, &item.TotalAmountCent, &item.PaidAmountCent, &snapshot, &expiresAt, &paidAt, &closedAt, &createdAt, &updatedAt)
	if err != nil {
		return Order{}, err
	}
	if err := json.Unmarshal(snapshot, &item.ProductSnapshot); err != nil {
		return Order{}, err
	}
	item.ExpiresAt = formatTime(expiresAt)
	item.PaidAt = optionalTime(paidAt)
	item.ClosedAt = optionalTime(closedAt)
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func randomBusinessNo(prefix string) (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(value)), nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func optionalTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	result := formatTime(value.Time)
	return &result
}
func boundedInt(raw string, fallback, min, max int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
