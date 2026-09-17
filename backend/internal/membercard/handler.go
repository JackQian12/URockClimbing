// Package membercard owns member-facing card queries.
package membercard

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db   *sql.DB
	auth *auth.Handler
}

type Card struct {
	ID               string  `json:"id"`
	CardNo           string  `json:"card_no"`
	ProductID        string  `json:"product_id"`
	ProductName      string  `json:"product_name"`
	ProductType      string  `json:"product_type"`
	TotalTimes       *uint   `json:"total_times"`
	RemainingTimes   *uint   `json:"remaining_times"`
	Status           string  `json:"status"`
	ActivatedAt      *string `json:"activated_at"`
	ExpiresAt        *string `json:"expires_at"`
	ValidityDays     uint    `json:"validity_days"`
	DailyUseLimit    uint    `json:"daily_use_limit"`
	RedemptionCount  uint    `json:"redemption_count"`
	LastRedemptionAt *string `json:"last_redemption_at"`
	CreatedAt        string  `json:"created_at"`
}

type Redemption struct {
	RedemptionNo    string `json:"redemption_no"`
	Times           uint   `json:"times"`
	BeforeRemaining *uint  `json:"before_remaining"`
	AfterRemaining  *uint  `json:"after_remaining"`
	RedeemedAt      string `json:"redeemed_at"`
	CardID          string `json:"card_id"`
	CardNo          string `json:"card_no"`
	ProductName     string `json:"product_name"`
}

func NewHandler(db *sql.DB, authenticator *auth.Handler) *Handler {
	return &Handler{db: db, auth: authenticator}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	h.expireCards(r, user.ID)

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	query := cardSelect + ` WHERE mc.user_id=?`
	args := []any{user.ID}
	if status != "" {
		if !validStatus(status) {
			respond.Error(w, r, http.StatusBadRequest, "INVALID_CARD_STATUS", "会员卡状态无效")
			return
		}
		query += ` AND mc.status=?`
		args = append(args, status)
	}
	query += ` ORDER BY FIELD(mc.status,'ACTIVE','PENDING_ACTIVATION','USED_UP','EXPIRED','REFUND_LOCKED','REFUNDED'),mc.created_at DESC,mc.id DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "会员卡加载失败")
		return
	}
	defer rows.Close()
	items := make([]Card, 0)
	for rows.Next() {
		item, err := scanCard(rows)
		if err != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "会员卡加载失败")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "会员卡加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	h.expireCards(r, user.ID)
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		respond.Error(w, r, 404, "CARD_NOT_FOUND", "会员卡不存在")
		return
	}
	item, err := scanCard(h.db.QueryRowContext(r.Context(), cardSelect+` WHERE mc.id=? AND mc.user_id=?`, id, user.ID))
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, 404, "CARD_NOT_FOUND", "会员卡不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "会员卡加载失败")
		return
	}
	respond.JSON(w, r, 200, item)
}

func (h *Handler) CardRedemptions(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil || !h.ownsCard(r, user.ID, id) {
		respond.Error(w, r, 404, "CARD_NOT_FOUND", "会员卡不存在")
		return
	}
	h.listRedemptions(w, r, user.ID, id)
}

func (h *Handler) Redemptions(w http.ResponseWriter, r *http.Request) {
	user, ok := h.registeredUser(w, r)
	if !ok {
		return
	}
	h.listRedemptions(w, r, user.ID, 0)
}

func (h *Handler) listRedemptions(w http.ResponseWriter, r *http.Request, userID, cardID uint64) {
	query := `SELECT rr.redemption_no,rr.times,rr.before_remaining,rr.after_remaining,rr.redeemed_at,mc.id,mc.card_no,mc.product_name FROM redemption_records rr JOIN member_cards mc ON mc.id=rr.member_card_id WHERE rr.user_id=?`
	args := []any{userID}
	if cardID != 0 {
		query += ` AND rr.member_card_id=?`
		args = append(args, cardID)
	}
	query += ` ORDER BY rr.redeemed_at DESC,rr.id DESC LIMIT 100`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]Redemption, 0)
	for rows.Next() {
		var item Redemption
		var cardID uint64
		var before, after sql.NullInt64
		var redeemedAt time.Time
		if err := rows.Scan(&item.RedemptionNo, &item.Times, &before, &after, &redeemedAt, &cardID, &item.CardNo, &item.ProductName); err != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
			return
		}
		item.CardID = strconv.FormatUint(cardID, 10)
		item.RedeemedAt = formatTime(redeemedAt)
		item.BeforeRemaining = nullUint(before)
		item.AfterRemaining = nullUint(after)
		items = append(items, item)
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": len(items)})
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

func (h *Handler) expireCards(r *http.Request, userID uint64) {
	_, _ = h.db.ExecContext(r.Context(), `UPDATE member_cards SET status='EXPIRED' WHERE user_id=? AND status='ACTIVE' AND expires_at<=UTC_TIMESTAMP(3)`, userID)
}

func (h *Handler) ownsCard(r *http.Request, userID, cardID uint64) bool {
	var one int
	return h.db.QueryRowContext(r.Context(), `SELECT 1 FROM member_cards WHERE id=? AND user_id=?`, cardID, userID).Scan(&one) == nil
}

const cardSelect = `SELECT mc.id,mc.card_no,mc.product_id,mc.product_name,mc.product_type,mc.total_times,mc.remaining_times,mc.status,mc.activated_at,mc.expires_at,cp.validity_days,cp.daily_use_limit,(SELECT COUNT(*) FROM redemption_records rr WHERE rr.member_card_id=mc.id),(SELECT MAX(rr.redeemed_at) FROM redemption_records rr WHERE rr.member_card_id=mc.id),mc.created_at FROM member_cards mc JOIN card_products cp ON cp.id=mc.product_id`

type scanner interface{ Scan(...any) error }

func scanCard(row scanner) (Card, error) {
	var item Card
	var id, productID uint64
	var total, remaining sql.NullInt64
	var activated, expires, last sql.NullTime
	var created time.Time
	err := row.Scan(&id, &item.CardNo, &productID, &item.ProductName, &item.ProductType, &total, &remaining, &item.Status, &activated, &expires, &item.ValidityDays, &item.DailyUseLimit, &item.RedemptionCount, &last, &created)
	if err != nil {
		return Card{}, err
	}
	item.ID = strconv.FormatUint(id, 10)
	item.ProductID = strconv.FormatUint(productID, 10)
	item.TotalTimes = nullUint(total)
	item.RemainingTimes = nullUint(remaining)
	item.ActivatedAt = nullTime(activated)
	item.ExpiresAt = nullTime(expires)
	item.LastRedemptionAt = nullTime(last)
	item.CreatedAt = formatTime(created)
	return item, nil
}

func parseID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if id == 0 {
		return 0, errors.New("invalid id")
	}
	return id, err
}
func validStatus(v string) bool {
	switch v {
	case "PENDING_ACTIVATION", "ACTIVE", "USED_UP", "EXPIRED", "REFUND_LOCKED", "REFUNDED":
		return true
	}
	return false
}
func nullUint(v sql.NullInt64) *uint {
	if !v.Valid {
		return nil
	}
	n := uint(v.Int64)
	return &n
}
func nullTime(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	s := formatTime(v.Time)
	return &s
}
func formatTime(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
