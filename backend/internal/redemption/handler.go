package redemption

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/respond"
)

const tokenTTL = 5 * time.Minute

var (
	errCardNotFound        = errors.New("card not found")
	errCardNotActive       = errors.New("card not active")
	errCardExpired         = errors.New("card expired")
	errCardUsedUp          = errors.New("card used up")
	errDailyLimit          = errors.New("daily limit reached")
	errTokenInvalid        = errors.New("token invalid")
	errTokenExpired        = errors.New("token expired")
	errTokenAlreadyUsed    = errors.New("token already used")
	errCheckinCodeInvalid  = errors.New("check-in code invalid")
	errCheckinCodeInactive = errors.New("check-in code inactive")
	errNoUsableCards       = errors.New("no usable cards")
	errPermission          = errors.New("insufficient permission")
	errUnauthenticated     = errors.New("unauthenticated")
)

type Handler struct {
	db   *sql.DB
	auth *auth.Handler
	now  func() time.Time
}

type Token struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type Preview struct {
	MemberNo       string  `json:"member_no"`
	Nickname       *string `json:"nickname"`
	PhoneLast4     *string `json:"phone_last4"`
	CardID         string  `json:"card_id"`
	CardNo         string  `json:"card_no"`
	ProductName    string  `json:"product_name"`
	ProductType    string  `json:"product_type"`
	Status         string  `json:"status"`
	RemainingTimes *uint   `json:"remaining_times"`
	ActivatedAt    *string `json:"activated_at"`
	ExpiresAt      *string `json:"expires_at"`
	TokenExpiresAt string  `json:"token_expires_at"`
}

type Result struct {
	RedemptionNo    string  `json:"redemption_no"`
	CardID          string  `json:"card_id"`
	CardNo          string  `json:"card_no"`
	ProductName     string  `json:"product_name"`
	ProductType     string  `json:"product_type"`
	BeforeRemaining *uint   `json:"before_remaining"`
	AfterRemaining  *uint   `json:"after_remaining"`
	CardStatus      string  `json:"card_status"`
	ActivatedAt     *string `json:"activated_at"`
	ExpiresAt       *string `json:"expires_at"`
	RedeemedAt      string  `json:"redeemed_at"`
}

type CheckinCard struct {
	CardID         string  `json:"card_id"`
	CardNo         string  `json:"card_no"`
	ProductName    string  `json:"product_name"`
	ProductType    string  `json:"product_type"`
	Status         string  `json:"status"`
	RemainingTimes *uint   `json:"remaining_times"`
	ActivatedAt    *string `json:"activated_at"`
	ExpiresAt      *string `json:"expires_at"`
}

type CheckinPreviewResult struct {
	CodeName string        `json:"code_name"`
	Cards    []CheckinCard `json:"cards"`
}

type CheckinResult struct {
	Result
	CodeName string `json:"code_name"`
}

func NewHandler(db *sql.DB, authenticator *auth.Handler) *Handler {
	return &Handler{db: db, auth: authenticator, now: time.Now}
}

func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	user, err := h.registered(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	cardID, err := parseID(r.PathValue("id"))
	if err != nil {
		h.writeError(w, r, errCardNotFound)
		return
	}
	token, err := h.createToken(r.Context(), user.ID, cardID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusCreated, token)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	_, err := h.staff(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	raw, err := decodeTokenRequest(w, r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	preview, err := h.preview(r.Context(), raw)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, preview)
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	operator, err := h.staff(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	raw, err := decodeTokenRequest(w, r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(requestID) < 8 || len(requestID) > 64 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST_ID", "核销请求无效，请重试")
		return
	}
	result, err := h.confirm(r.Context(), operator.ID, raw, requestID, r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) Today(w http.ResponseWriter, r *http.Request) {
	operator, err := h.staff(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	start, end := shanghaiDay(h.now().UTC())
	rows, err := h.db.QueryContext(r.Context(), `SELECT rr.redemption_no,mc.card_no,mc.product_name,rr.before_remaining,rr.after_remaining,rr.redeemed_at FROM redemption_records rr JOIN member_cards mc ON mc.id=rr.member_card_id WHERE rr.operator_user_id=? AND rr.redeemed_at>=? AND rr.redeemed_at<? ORDER BY rr.redeemed_at DESC,rr.id DESC`, operator.ID, start, end)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "今日核销记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var no, cardNo, product string
		var before, after sql.NullInt64
		var redeemed time.Time
		if rows.Scan(&no, &cardNo, &product, &before, &after, &redeemed) != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "今日核销记录加载失败")
			return
		}
		items = append(items, map[string]any{"redemption_no": no, "card_no": cardNo, "product_name": product, "before_remaining": nullUint(before), "after_remaining": nullUint(after), "redeemed_at": formatTime(redeemed)})
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) CheckinPreview(w http.ResponseWriter, r *http.Request) {
	user, err := h.registered(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	raw, err := decodeCheckinTokenRequest(w, r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	result, err := h.checkinPreview(r.Context(), user.ID, raw)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) CheckinConfirm(w http.ResponseWriter, r *http.Request) {
	user, err := h.registered(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var input struct {
		Token  string `json:"token"`
		CardID string `json:"card_id"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil {
		h.writeError(w, r, errCheckinCodeInvalid)
		return
	}
	raw, err := normalizeCheckinToken(input.Token)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	cardID, err := parseID(input.CardID)
	if err != nil {
		h.writeError(w, r, errCardNotFound)
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(requestID) < 8 || len(requestID) > 64 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST_ID", "签到请求无效，请重试")
		return
	}
	result, err := h.checkinConfirm(r.Context(), user.ID, cardID, raw, requestID, r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) checkinPreview(ctx context.Context, userID uint64, raw string) (CheckinPreviewResult, error) {
	var codeName, codeStatus string
	err := h.db.QueryRowContext(ctx, `SELECT name,status FROM checkin_codes WHERE token_hash=?`, hashToken(raw)).Scan(&codeName, &codeStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckinPreviewResult{}, errCheckinCodeInvalid
	}
	if err != nil {
		return CheckinPreviewResult{}, err
	}
	if codeStatus != "ACTIVE" {
		return CheckinPreviewResult{}, errCheckinCodeInactive
	}
	rows, err := h.db.QueryContext(ctx, `SELECT mc.id,mc.card_no,mc.product_name,mc.product_type,mc.status,mc.remaining_times,mc.activated_at,mc.expires_at,cp.daily_use_limit FROM member_cards mc JOIN card_products cp ON cp.id=mc.product_id WHERE mc.user_id=? AND mc.status IN ('PENDING_ACTIVATION','ACTIVE') ORDER BY (mc.status='ACTIVE') DESC,mc.created_at ASC,mc.id ASC`, userID)
	if err != nil {
		return CheckinPreviewResult{}, err
	}
	type candidate struct {
		id                         uint64
		cardNo, name, kind, status string
		remaining                  sql.NullInt64
		activated, expires         sql.NullTime
		dailyLimit                 uint
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.cardNo, &c.name, &c.kind, &c.status, &c.remaining, &c.activated, &c.expires, &c.dailyLimit); err != nil {
			_ = rows.Close()
			return CheckinPreviewResult{}, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Close(); err != nil {
		return CheckinPreviewResult{}, err
	}
	if err := rows.Err(); err != nil {
		return CheckinPreviewResult{}, err
	}
	now := h.now().UTC()
	cards := make([]CheckinCard, 0, len(candidates))
	for _, c := range candidates {
		if err := validateCard(ctx, h.db, c.id, c.kind, c.status, c.remaining, c.expires, c.dailyLimit, now); err != nil {
			if errors.Is(err, errCardExpired) || errors.Is(err, errCardUsedUp) || errors.Is(err, errCardNotActive) || errors.Is(err, errDailyLimit) {
				continue
			}
			return CheckinPreviewResult{}, err
		}
		cards = append(cards, CheckinCard{CardID: strconv.FormatUint(c.id, 10), CardNo: c.cardNo, ProductName: c.name, ProductType: c.kind, Status: c.status, RemainingTimes: nullUint(c.remaining), ActivatedAt: nullTime(c.activated), ExpiresAt: nullTime(c.expires)})
	}
	if len(cards) == 0 {
		return CheckinPreviewResult{}, errNoUsableCards
	}
	return CheckinPreviewResult{CodeName: codeName, Cards: cards}, nil
}

func (h *Handler) checkinConfirm(ctx context.Context, userID, cardID uint64, raw, requestID string, request *http.Request) (CheckinResult, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return CheckinResult{}, err
	}
	defer tx.Rollback()
	if existing, err := getCheckinResult(ctx, tx, userID, requestID); err == nil {
		_ = tx.Commit()
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CheckinResult{}, err
	}
	var codeID uint64
	var codeName, codeStatus string
	err = tx.QueryRowContext(ctx, `SELECT id,name,status FROM checkin_codes WHERE token_hash=? FOR UPDATE`, hashToken(raw)).Scan(&codeID, &codeName, &codeStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckinResult{}, errCheckinCodeInvalid
	}
	if err != nil {
		return CheckinResult{}, err
	}
	if codeStatus != "ACTIVE" {
		return CheckinResult{}, errCheckinCodeInactive
	}
	// Another request with the same idempotency key may have completed while this
	// transaction was waiting for the reusable code row lock.
	if existing, err := getCheckinResultLocked(ctx, tx, userID, requestID); err == nil {
		_ = tx.Commit()
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CheckinResult{}, err
	}

	var cardNo, productName, productType, status string
	var total, remaining sql.NullInt64
	var activated, expires sql.NullTime
	var validityDays, dailyLimit uint
	err = tx.QueryRowContext(ctx, `SELECT mc.card_no,mc.product_name,mc.product_type,mc.total_times,mc.remaining_times,mc.status,mc.activated_at,mc.expires_at,cp.validity_days,cp.daily_use_limit FROM member_cards mc JOIN card_products cp ON cp.id=mc.product_id WHERE mc.id=? AND mc.user_id=? FOR UPDATE`, cardID, userID).Scan(&cardNo, &productName, &productType, &total, &remaining, &status, &activated, &expires, &validityDays, &dailyLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckinResult{}, errCardNotFound
	}
	if err != nil {
		return CheckinResult{}, err
	}
	now := h.now().UTC()
	if err := validateCard(ctx, tx, cardID, productType, status, remaining, expires, dailyLimit, now); err != nil {
		return CheckinResult{}, err
	}
	activatedAt, expiresAt := activated, expires
	if status == "PENDING_ACTIVATION" {
		activatedAt = sql.NullTime{Time: now, Valid: true}
		expiresAt = sql.NullTime{Time: now.AddDate(0, 0, int(validityDays)), Valid: true}
	}
	newStatus := "ACTIVE"
	var before, after sql.NullInt64
	if productType == "COUNT_CARD" {
		before = remaining
		after = sql.NullInt64{Int64: remaining.Int64 - 1, Valid: true}
		if after.Int64 == 0 {
			newStatus = "USED_UP"
		}
		_, err = tx.ExecContext(ctx, `UPDATE member_cards SET remaining_times=?,status=?,activated_at=?,expires_at=?,version=version+1 WHERE id=?`, after.Int64, newStatus, activatedAt.Time, expiresAt.Time, cardID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE member_cards SET status='ACTIVE',activated_at=?,expires_at=?,version=version+1 WHERE id=?`, activatedAt.Time, expiresAt.Time, cardID)
	}
	if err != nil {
		return CheckinResult{}, err
	}
	redemptionNo, err := businessNo("URRED")
	if err != nil {
		return CheckinResult{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO redemption_records(redemption_no,member_card_id,user_id,operator_user_id,token_id,checkin_code_id,redemption_mode,times,before_remaining,after_remaining,redeemed_at,request_id) VALUES(?,?,?,?,NULL,?,'MEMBER_SELF_SCAN',1,?,?,?,?)`, redemptionNo, cardID, userID, userID, codeID, nullableInt(before), nullableInt(after), now, requestID)
	if err != nil {
		return CheckinResult{}, err
	}
	afterSnapshot, _ := json.Marshal(map[string]any{"redemption_no": redemptionNo, "checkin_code_id": codeID, "card_status": newStatus, "before_remaining": nullableInt(before), "after_remaining": nullableInt(after)})
	requestLogID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
	if requestLogID == "" {
		requestLogID = requestID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,after_snapshot,request_ip,request_id) VALUES(?,'MEMBER_SELF_CHECKIN','MEMBER_CARD',?,?,?,?)`, userID, cardNo, afterSnapshot, request.RemoteAddr, requestLogID); err != nil {
		return CheckinResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CheckinResult{}, err
	}
	return CheckinResult{Result: Result{RedemptionNo: redemptionNo, CardID: strconv.FormatUint(cardID, 10), CardNo: cardNo, ProductName: productName, ProductType: productType, BeforeRemaining: nullUint(before), AfterRemaining: nullUint(after), CardStatus: newStatus, ActivatedAt: nullTime(activatedAt), ExpiresAt: nullTime(expiresAt), RedeemedAt: formatTime(now)}, CodeName: codeName}, nil
}

func getCheckinResult(ctx context.Context, tx *sql.Tx, userID uint64, requestID string) (CheckinResult, error) {
	return scanCheckinResult(tx.QueryRowContext(ctx, `SELECT rr.redemption_no,mc.id,mc.card_no,mc.product_name,mc.product_type,rr.before_remaining,rr.after_remaining,mc.status,mc.activated_at,mc.expires_at,rr.redeemed_at,cc.name FROM redemption_records rr JOIN member_cards mc ON mc.id=rr.member_card_id JOIN checkin_codes cc ON cc.id=rr.checkin_code_id WHERE rr.operator_user_id=? AND rr.request_id=? AND rr.redemption_mode='MEMBER_SELF_SCAN'`, userID, requestID))
}

func getCheckinResultLocked(ctx context.Context, tx *sql.Tx, userID uint64, requestID string) (CheckinResult, error) {
	return scanCheckinResult(tx.QueryRowContext(ctx, `SELECT rr.redemption_no,mc.id,mc.card_no,mc.product_name,mc.product_type,rr.before_remaining,rr.after_remaining,mc.status,mc.activated_at,mc.expires_at,rr.redeemed_at,cc.name FROM redemption_records rr JOIN member_cards mc ON mc.id=rr.member_card_id JOIN checkin_codes cc ON cc.id=rr.checkin_code_id WHERE rr.operator_user_id=? AND rr.request_id=? AND rr.redemption_mode='MEMBER_SELF_SCAN' FOR UPDATE`, userID, requestID))
}

func scanCheckinResult(row interface{ Scan(...any) error }) (CheckinResult, error) {
	var item CheckinResult
	var cardID uint64
	var before, after sql.NullInt64
	var activated, expires sql.NullTime
	var redeemed time.Time
	err := row.Scan(&item.RedemptionNo, &cardID, &item.CardNo, &item.ProductName, &item.ProductType, &before, &after, &item.CardStatus, &activated, &expires, &redeemed, &item.CodeName)
	if err != nil {
		return CheckinResult{}, err
	}
	item.CardID = strconv.FormatUint(cardID, 10)
	item.BeforeRemaining = nullUint(before)
	item.AfterRemaining = nullUint(after)
	item.ActivatedAt = nullTime(activated)
	item.ExpiresAt = nullTime(expires)
	item.RedeemedAt = formatTime(redeemed)
	return item, nil
}

func (h *Handler) createToken(ctx context.Context, userID, cardID uint64) (Token, error) {
	now := h.now().UTC()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return Token{}, err
	}
	defer tx.Rollback()
	var productType, status string
	var remaining sql.NullInt64
	var expires sql.NullTime
	var dailyLimit uint
	err = tx.QueryRowContext(ctx, `SELECT mc.product_type,mc.status,mc.remaining_times,mc.expires_at,cp.daily_use_limit FROM member_cards mc JOIN card_products cp ON cp.id=mc.product_id WHERE mc.id=? AND mc.user_id=? FOR UPDATE`, cardID, userID).Scan(&productType, &status, &remaining, &expires, &dailyLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, errCardNotFound
	}
	if err != nil {
		return Token{}, err
	}
	if err := validateCard(ctx, tx, cardID, productType, status, remaining, expires, dailyLimit, now); err != nil {
		return Token{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE redemption_tokens SET status=IF(expires_at<=?,'EXPIRED','REVOKED') WHERE member_card_id=? AND status='ACTIVE'`, now, cardID); err != nil {
		return Token{}, err
	}
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return Token{}, err
	}
	raw := base64.RawURLEncoding.EncodeToString(rawBytes)
	hash := hashToken(raw)
	expiresAt := now.Add(tokenTTL)
	if _, err := tx.ExecContext(ctx, `INSERT INTO redemption_tokens(token_hash,member_card_id,user_id,status,expires_at) VALUES(?,?,?,'ACTIVE',?)`, hash, cardID, userID, expiresAt); err != nil {
		return Token{}, err
	}
	if err := tx.Commit(); err != nil {
		return Token{}, err
	}
	return Token{Token: raw, ExpiresAt: formatTime(expiresAt)}, nil
}

func (h *Handler) preview(ctx context.Context, raw string) (Preview, error) {
	now := h.now().UTC()
	var item Preview
	var cardID uint64
	var nickname, phone sql.NullString
	var remaining sql.NullInt64
	var activated, expires, tokenExpires sql.NullTime
	var tokenStatus string
	err := h.db.QueryRowContext(ctx, `SELECT u.member_no,u.nickname,mp.phone_last4,mc.id,mc.card_no,mc.product_name,mc.product_type,mc.status,mc.remaining_times,mc.activated_at,mc.expires_at,rt.status,rt.expires_at FROM redemption_tokens rt JOIN member_cards mc ON mc.id=rt.member_card_id JOIN users u ON u.id=rt.user_id LEFT JOIN member_profiles mp ON mp.user_id=u.id WHERE rt.token_hash=?`, hashToken(raw)).Scan(&item.MemberNo, &nickname, &phone, &cardID, &item.CardNo, &item.ProductName, &item.ProductType, &item.Status, &remaining, &activated, &expires, &tokenStatus, &tokenExpires)
	if errors.Is(err, sql.ErrNoRows) {
		return Preview{}, errTokenInvalid
	}
	if err != nil {
		return Preview{}, err
	}
	if tokenStatus != "ACTIVE" {
		return Preview{}, errTokenAlreadyUsed
	}
	if !tokenExpires.Valid || !tokenExpires.Time.After(now) {
		_, _ = h.db.ExecContext(ctx, `UPDATE redemption_tokens SET status='EXPIRED' WHERE token_hash=? AND status='ACTIVE'`, hashToken(raw))
		return Preview{}, errTokenExpired
	}
	if err := validateSimpleCard(item.ProductType, item.Status, remaining, expires, now); err != nil {
		return Preview{}, err
	}
	item.CardID = strconv.FormatUint(cardID, 10)
	item.Nickname = nullString(nickname)
	item.PhoneLast4 = nullString(phone)
	item.RemainingTimes = nullUint(remaining)
	item.ActivatedAt = nullTime(activated)
	item.ExpiresAt = nullTime(expires)
	item.TokenExpiresAt = formatTime(tokenExpires.Time)
	return item, nil
}

func (h *Handler) confirm(ctx context.Context, operatorID uint64, raw, requestID string, request *http.Request) (Result, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if existing, err := getResult(ctx, tx, operatorID, requestID); err == nil {
		_ = tx.Commit()
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}

	now := h.now().UTC()
	var tokenID, cardID, userID uint64
	var tokenStatus string
	var tokenExpires time.Time
	err = tx.QueryRowContext(ctx, `SELECT id,member_card_id,user_id,status,expires_at FROM redemption_tokens WHERE token_hash=? FOR UPDATE`, hashToken(raw)).Scan(&tokenID, &cardID, &userID, &tokenStatus, &tokenExpires)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, errTokenInvalid
	}
	if err != nil {
		return Result{}, err
	}
	if tokenStatus != "ACTIVE" {
		if existing, existingErr := getResult(ctx, tx, operatorID, requestID); existingErr == nil {
			_ = tx.Commit()
			return existing, nil
		}
		return Result{}, errTokenAlreadyUsed
	}
	if !tokenExpires.After(now) {
		_, _ = tx.ExecContext(ctx, `UPDATE redemption_tokens SET status='EXPIRED' WHERE id=?`, tokenID)
		_ = tx.Commit()
		return Result{}, errTokenExpired
	}

	var cardNo, productName, productType, status string
	var total, remaining sql.NullInt64
	var activated, expires sql.NullTime
	var validityDays, dailyLimit uint
	err = tx.QueryRowContext(ctx, `SELECT mc.card_no,mc.product_name,mc.product_type,mc.total_times,mc.remaining_times,mc.status,mc.activated_at,mc.expires_at,cp.validity_days,cp.daily_use_limit FROM member_cards mc JOIN card_products cp ON cp.id=mc.product_id WHERE mc.id=? AND mc.user_id=? FOR UPDATE`, cardID, userID).Scan(&cardNo, &productName, &productType, &total, &remaining, &status, &activated, &expires, &validityDays, &dailyLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, errCardNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if err := validateCard(ctx, tx, cardID, productType, status, remaining, expires, dailyLimit, now); err != nil {
		return Result{}, err
	}

	activatedAt, expiresAt := activated, expires
	if status == "PENDING_ACTIVATION" {
		activatedAt = sql.NullTime{Time: now, Valid: true}
		expiresAt = sql.NullTime{Time: now.AddDate(0, 0, int(validityDays)), Valid: true}
	}
	newStatus := "ACTIVE"
	var before, after sql.NullInt64
	if productType == "COUNT_CARD" {
		before = remaining
		after = sql.NullInt64{Int64: remaining.Int64 - 1, Valid: true}
		if after.Int64 == 0 {
			newStatus = "USED_UP"
		}
		_, err = tx.ExecContext(ctx, `UPDATE member_cards SET remaining_times=?,status=?,activated_at=?,expires_at=?,version=version+1 WHERE id=?`, after.Int64, newStatus, activatedAt.Time, expiresAt.Time, cardID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE member_cards SET status='ACTIVE',activated_at=?,expires_at=?,version=version+1 WHERE id=?`, activatedAt.Time, expiresAt.Time, cardID)
	}
	if err != nil {
		return Result{}, err
	}
	redemptionNo, err := businessNo("URRED")
	if err != nil {
		return Result{}, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO redemption_records(redemption_no,member_card_id,user_id,operator_user_id,token_id,times,before_remaining,after_remaining,redeemed_at,request_id) VALUES(?,?,?,?,?,1,?,?,?,?)`, redemptionNo, cardID, userID, operatorID, tokenID, nullableInt(before), nullableInt(after), now, requestID)
	if err != nil {
		return Result{}, err
	}
	_ = res
	if _, err := tx.ExecContext(ctx, `UPDATE redemption_tokens SET status='USED',used_at=? WHERE id=? AND status='ACTIVE'`, now, tokenID); err != nil {
		return Result{}, err
	}
	afterSnapshot, _ := json.Marshal(map[string]any{"redemption_no": redemptionNo, "card_status": newStatus, "before_remaining": nullableInt(before), "after_remaining": nullableInt(after)})
	requestLogID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
	if requestLogID == "" {
		requestLogID = requestID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,after_snapshot,request_ip,request_id) VALUES(?,'CARD_REDEEM','MEMBER_CARD',?,?,?,?)`, operatorID, cardNo, afterSnapshot, request.RemoteAddr, requestLogID); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{RedemptionNo: redemptionNo, CardID: strconv.FormatUint(cardID, 10), CardNo: cardNo, ProductName: productName, ProductType: productType, BeforeRemaining: nullUint(before), AfterRemaining: nullUint(after), CardStatus: newStatus, ActivatedAt: nullTime(activatedAt), ExpiresAt: nullTime(expiresAt), RedeemedAt: formatTime(now)}, nil
}

func getResult(ctx context.Context, tx *sql.Tx, operatorID uint64, requestID string) (Result, error) {
	var item Result
	var cardID uint64
	var before, after sql.NullInt64
	var activated, expires sql.NullTime
	var redeemed time.Time
	err := tx.QueryRowContext(ctx, `SELECT rr.redemption_no,mc.id,mc.card_no,mc.product_name,mc.product_type,rr.before_remaining,rr.after_remaining,mc.status,mc.activated_at,mc.expires_at,rr.redeemed_at FROM redemption_records rr JOIN member_cards mc ON mc.id=rr.member_card_id WHERE rr.operator_user_id=? AND rr.request_id=?`, operatorID, requestID).Scan(&item.RedemptionNo, &cardID, &item.CardNo, &item.ProductName, &item.ProductType, &before, &after, &item.CardStatus, &activated, &expires, &redeemed)
	if err != nil {
		return Result{}, err
	}
	item.CardID = strconv.FormatUint(cardID, 10)
	item.BeforeRemaining = nullUint(before)
	item.AfterRemaining = nullUint(after)
	item.ActivatedAt = nullTime(activated)
	item.ExpiresAt = nullTime(expires)
	item.RedeemedAt = formatTime(redeemed)
	return item, nil
}

func validateCard(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, cardID uint64, productType, status string, remaining sql.NullInt64, expires sql.NullTime, dailyLimit uint, now time.Time) error {
	if err := validateSimpleCard(productType, status, remaining, expires, now); err != nil {
		return err
	}
	if productType == "TIME_PASS" && status == "ACTIVE" {
		start, end := shanghaiDay(now)
		var count uint
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM redemption_records WHERE member_card_id=? AND redeemed_at>=? AND redeemed_at<?`, cardID, start, end).Scan(&count); err != nil {
			return err
		}
		if count >= dailyLimit {
			return errDailyLimit
		}
	}
	return nil
}

func validateSimpleCard(productType, status string, remaining sql.NullInt64, expires sql.NullTime, now time.Time) error {
	if status == "EXPIRED" || (status == "ACTIVE" && expires.Valid && !expires.Time.After(now)) {
		return errCardExpired
	}
	if status == "USED_UP" || (productType == "COUNT_CARD" && (!remaining.Valid || remaining.Int64 <= 0)) {
		return errCardUsedUp
	}
	if status != "ACTIVE" && status != "PENDING_ACTIVATION" {
		return errCardNotActive
	}
	return nil
}

func (h *Handler) registered(r *http.Request) (auth.CurrentUser, error) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		return user, errUnauthenticated
	}
	if !user.Registered {
		return user, errPermission
	}
	return user, nil
}
func (h *Handler) staff(r *http.Request) (auth.CurrentUser, error) {
	user, err := h.registered(r)
	if err != nil {
		return user, err
	}
	if user.Role != "STAFF" && user.Role != "ADMIN" {
		return user, errPermission
	}
	return user, nil
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errCardNotFound):
		respond.Error(w, r, 404, "CARD_NOT_FOUND", "会员卡不存在")
	case errors.Is(err, errCardExpired):
		respond.Error(w, r, 409, "CARD_EXPIRED", "会员卡已过期")
	case errors.Is(err, errCardUsedUp):
		respond.Error(w, r, 409, "CARD_USED_UP", "会员卡次数已用完")
	case errors.Is(err, errCardNotActive):
		respond.Error(w, r, 409, "CARD_NOT_ACTIVE", "会员卡当前不可使用")
	case errors.Is(err, errDailyLimit):
		respond.Error(w, r, 409, "DAILY_USE_LIMIT_REACHED", "期限卡今日已使用")
	case errors.Is(err, errTokenInvalid):
		respond.Error(w, r, 400, "TOKEN_INVALID", "核销码无效")
	case errors.Is(err, errTokenExpired):
		respond.Error(w, r, 409, "TOKEN_EXPIRED", "核销码已过期，请重新生成")
	case errors.Is(err, errTokenAlreadyUsed):
		respond.Error(w, r, 409, "TOKEN_ALREADY_USED", "核销码已使用或已失效")
	case errors.Is(err, errCheckinCodeInvalid):
		respond.Error(w, r, 400, "CHECKIN_CODE_INVALID", "签到码无效，请扫描场馆提供的二维码")
	case errors.Is(err, errCheckinCodeInactive):
		respond.Error(w, r, 409, "CHECKIN_CODE_INACTIVE", "该签到码已停用，请联系工作人员")
	case errors.Is(err, errNoUsableCards):
		respond.Error(w, r, 409, "NO_USABLE_CARDS", "暂无可用于签到或核销的会员卡")
	case errors.Is(err, errPermission):
		respond.Error(w, r, 403, "INSUFFICIENT_PERMISSION", "无权执行此操作")
	case errors.Is(err, errUnauthenticated):
		respond.Error(w, r, 401, "UNAUTHENTICATED", "请先登录")
	default:
		respond.Error(w, r, 500, "INTERNAL_ERROR", "核销操作失败，请稍后重试")
	}
}

func decodeTokenRequest(w http.ResponseWriter, r *http.Request) (string, error) {
	var input struct {
		Token string `json:"token"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil {
		return "", errTokenInvalid
	}
	input.Token = strings.TrimSpace(input.Token)
	if len(input.Token) < 32 || len(input.Token) > 128 {
		return "", errTokenInvalid
	}
	return input.Token, nil
}

func decodeCheckinTokenRequest(w http.ResponseWriter, r *http.Request) (string, error) {
	var input struct {
		Token string `json:"token"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil {
		return "", errCheckinCodeInvalid
	}
	return normalizeCheckinToken(input.Token)
}

func normalizeCheckinToken(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "urock-checkin:") {
		return "", errCheckinCodeInvalid
	}
	raw := strings.TrimSpace(strings.TrimPrefix(value, "urock-checkin:"))
	if len(raw) < 32 || len(raw) > 128 {
		return "", errCheckinCodeInvalid
	}
	return raw, nil
}
func parseID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0, errCardNotFound
	}
	return id, nil
}
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func businessNo(prefix string) (string, error) {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(value)), nil
}
func shanghaiDay(now time.Time) (time.Time, time.Time) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start.UTC(), start.Add(24 * time.Hour).UTC()
}
func nullableInt(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}
func nullUint(v sql.NullInt64) *uint {
	if !v.Valid {
		return nil
	}
	n := uint(v.Int64)
	return &n
}
func nullString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}
func nullTime(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	s := formatTime(v.Time)
	return &s
}
func formatTime(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
