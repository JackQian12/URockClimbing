package admincard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/respond"
)

var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Handler struct {
	db   *sql.DB
	auth *adminauth.Handler
}

type Product struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	ShortDescription string  `json:"short_description"`
	Description      string  `json:"description"`
	TotalTimes       uint    `json:"total_times"`
	ValidityDays     uint    `json:"validity_days"`
	ActivationMode   string  `json:"activation_mode"`
	PriceCent        uint64  `json:"price_cent"`
	ListPriceCent    *uint64 `json:"list_price_cent"`
	PurchaseLimit    *uint   `json:"purchase_limit"`
	DailyUseLimit    uint    `json:"daily_use_limit"`
	Transferable     bool    `json:"transferable"`
	Rules            string  `json:"rules"`
	Badge            *string `json:"badge"`
	ThemeColor       string  `json:"theme_color"`
	Status           string  `json:"status"`
	SortOrder        int     `json:"sort_order"`
	Version          uint    `json:"version"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type productInput struct {
	Name             string  `json:"name"`
	ShortDescription string  `json:"short_description"`
	Description      string  `json:"description"`
	TotalTimes       uint    `json:"total_times"`
	ValidityDays     uint    `json:"validity_days"`
	ActivationMode   string  `json:"activation_mode"`
	PriceCent        uint64  `json:"price_cent"`
	ListPriceCent    *uint64 `json:"list_price_cent"`
	PurchaseLimit    *uint   `json:"purchase_limit"`
	DailyUseLimit    uint    `json:"daily_use_limit"`
	Transferable     bool    `json:"transferable"`
	Rules            string  `json:"rules"`
	Badge            *string `json:"badge"`
	ThemeColor       string  `json:"theme_color"`
	SortOrder        int     `json:"sort_order"`
	Version          uint    `json:"version"`
}

func NewHandler(db *sql.DB, auth *adminauth.Handler) *Handler {
	return &Handler{db: db, auth: auth}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), productColumns+` ORDER BY sort_order DESC, id DESC`)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品列表加载失败")
		return
	}
	defer rows.Close()
	products := make([]Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品列表加载失败")
			return
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品列表加载失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, map[string]any{"items": products, "total": len(products)})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	input, ok := decodeInput(w, r, false)
	if !ok {
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品创建失败")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO card_products
		(name, short_description, description, total_times, validity_days, activation_mode,
		 price_cent, list_price_cent, purchase_limit, daily_use_limit, transferable, rules,
		 badge, theme_color, status, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'DRAFT', ?)`,
		input.Name, input.ShortDescription, input.Description, input.TotalTimes, input.ValidityDays,
		input.ActivationMode, input.PriceCent, input.ListPriceCent, input.PurchaseLimit,
		input.DailyUseLimit, input.Transferable, input.Rules, input.Badge, input.ThemeColor, input.SortOrder,
	)
	var id int64
	if err == nil {
		id, err = result.LastInsertId()
	}
	if err == nil {
		err = writeAudit(r.Context(), tx, session.UserID, "CARD_PRODUCT_CREATE", strconv.FormatInt(id, 10), nil, input, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品创建失败")
		return
	}
	product, err := h.get(r.Context(), uint64(id))
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品创建成功，但读取失败")
		return
	}
	respond.JSON(w, r, http.StatusCreated, product)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	input, ok := decodeInput(w, r, true)
	if !ok {
		return
	}
	before, err := h.get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "CARD_PRODUCT_NOT_FOUND", "次卡商品不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品读取失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `
			UPDATE card_products SET name = ?, short_description = ?, description = ?, total_times = ?,
			validity_days = ?, activation_mode = ?, price_cent = ?, list_price_cent = ?, purchase_limit = ?,
			daily_use_limit = ?, transferable = ?, rules = ?, badge = ?, theme_color = ?, sort_order = ?, version = version + 1
			WHERE id = ? AND version = ?`,
			input.Name, input.ShortDescription, input.Description, input.TotalTimes, input.ValidityDays,
			input.ActivationMode, input.PriceCent, input.ListPriceCent, input.PurchaseLimit,
			input.DailyUseLimit, input.Transferable, input.Rules, input.Badge, input.ThemeColor,
			input.SortOrder, id, input.Version,
		)
		if err == nil {
			var affected int64
			affected, err = result.RowsAffected()
			if err == nil && affected != 1 {
				err = errVersionConflict
			}
		}
	}
	if err == nil {
		err = writeAudit(r.Context(), tx, session.UserID, "CARD_PRODUCT_UPDATE", strconv.FormatUint(id, 10), before, input, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, errVersionConflict) {
			respond.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "商品已被其他管理员修改，请刷新后重试")
			return
		}
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品保存失败")
		return
	}
	product, err := h.get(r.Context(), id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品保存成功，但读取失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, product)
}

func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Status  string `json:"status"`
		Version uint   `json:"version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Version == 0 || (input.Status != "ON_SALE" && input.Status != "OFF_SALE") {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_CARD_PRODUCT_STATUS", "请选择有效的商品状态")
		return
	}
	before, err := h.get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "CARD_PRODUCT_NOT_FOUND", "次卡商品不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品读取失败")
		return
	}
	if !validStatusTransition(before.Status, input.Status) {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_STATUS_TRANSITION", "当前商品状态不允许此操作")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `UPDATE card_products SET status = ?, version = version + 1 WHERE id = ? AND version = ? AND status = ?`, input.Status, id, input.Version, before.Status)
		if err == nil {
			var affected int64
			affected, err = result.RowsAffected()
			if err == nil && affected != 1 {
				err = errVersionConflict
			}
		}
	}
	if err == nil {
		action := "CARD_PRODUCT_PUBLISH"
		if input.Status == "OFF_SALE" {
			action = "CARD_PRODUCT_UNPUBLISH"
		}
		err = writeAudit(r.Context(), tx, session.UserID, action, strconv.FormatUint(id, 10), before, input, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, errVersionConflict) {
			respond.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "商品状态已变化，请刷新后重试")
			return
		}
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "商品状态修改失败")
		return
	}
	product, err := h.get(r.Context(), id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "状态修改成功，但读取失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, product)
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, mutation bool) (adminauth.Session, bool) {
	session, ok := h.auth.Authenticate(r)
	if !ok {
		respond.Error(w, r, http.StatusUnauthorized, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return adminauth.Session{}, false
	}
	if session.MustChangePassword {
		respond.Error(w, r, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "请先修改初始密码")
		return adminauth.Session{}, false
	}
	if mutation && !h.auth.ValidCSRF(session, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, http.StatusForbidden, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return adminauth.Session{}, false
	}
	return session, true
}

func decodeInput(w http.ResponseWriter, r *http.Request, requireVersion bool) (productInput, bool) {
	var input productInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "请输入有效的商品信息")
		return productInput{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ShortDescription = strings.TrimSpace(input.ShortDescription)
	input.Description = strings.TrimSpace(input.Description)
	input.Rules = strings.TrimSpace(input.Rules)
	input.ThemeColor = strings.ToUpper(strings.TrimSpace(input.ThemeColor))
	if input.Badge != nil {
		value := strings.TrimSpace(*input.Badge)
		if value == "" {
			input.Badge = nil
		} else {
			input.Badge = &value
		}
	}
	if message := validateInput(input, requireVersion); message != "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_CARD_PRODUCT", message)
		return productInput{}, false
	}
	return input, true
}

func validateInput(input productInput, requireVersion bool) string {
	if utf8.RuneCountInString(input.Name) < 2 || utf8.RuneCountInString(input.Name) > 100 {
		return "商品名称需为 2 至 100 个字符"
	}
	if utf8.RuneCountInString(input.ShortDescription) > 255 || utf8.RuneCountInString(input.Description) > 5000 || utf8.RuneCountInString(input.Rules) > 5000 {
		return "商品介绍或使用规则过长"
	}
	if input.TotalTimes == 0 || input.TotalTimes > 1000 {
		return "可用次数需为 1 至 1000 次"
	}
	if input.ValidityDays == 0 || input.ValidityDays > 3650 {
		return "有效期需为 1 至 3650 天"
	}
	if input.ActivationMode != "PURCHASE" && input.ActivationMode != "FIRST_USE" {
		return "请选择有效的生效方式"
	}
	if input.PriceCent == 0 || input.PriceCent > 100000000 {
		return "售价必须大于 0"
	}
	if input.ListPriceCent != nil && *input.ListPriceCent < input.PriceCent {
		return "划线价不能低于售价"
	}
	if input.PurchaseLimit != nil && (*input.PurchaseLimit == 0 || *input.PurchaseLimit > 100) {
		return "每人限购需为 1 至 100 张"
	}
	if input.DailyUseLimit == 0 || input.DailyUseLimit > 20 {
		return "每日可用次数需为 1 至 20 次"
	}
	if input.Badge != nil && utf8.RuneCountInString(*input.Badge) > 32 {
		return "商品标签不能超过 32 个字符"
	}
	if !colorPattern.MatchString(input.ThemeColor) {
		return "主题色需为有效的十六进制颜色"
	}
	if requireVersion && input.Version == 0 {
		return "商品版本无效，请刷新后重试"
	}
	return ""
}

func pathID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_CARD_PRODUCT_ID", "商品编号无效")
		return 0, false
	}
	return id, true
}

func validStatusTransition(current, target string) bool {
	return (current == "DRAFT" && target == "ON_SALE") ||
		(current == "ON_SALE" && target == "OFF_SALE") ||
		(current == "OFF_SALE" && target == "ON_SALE")
}

const productColumns = `
	SELECT id, name, short_description, description, total_times, validity_days, activation_mode,
	       price_cent, list_price_cent, purchase_limit, daily_use_limit, transferable, rules,
	       badge, theme_color, status, sort_order, version, created_at, updated_at
	FROM card_products`

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(row scanner) (Product, error) {
	var product Product
	var id uint64
	var listPrice sql.NullInt64
	var purchaseLimit sql.NullInt64
	var badge sql.NullString
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &product.Name, &product.ShortDescription, &product.Description,
		&product.TotalTimes, &product.ValidityDays, &product.ActivationMode, &product.PriceCent,
		&listPrice, &purchaseLimit, &product.DailyUseLimit, &product.Transferable, &product.Rules,
		&badge, &product.ThemeColor, &product.Status, &product.SortOrder, &product.Version,
		&createdAt, &updatedAt)
	if err != nil {
		return Product{}, err
	}
	product.ID = strconv.FormatUint(id, 10)
	if listPrice.Valid {
		value := uint64(listPrice.Int64)
		product.ListPriceCent = &value
	}
	if purchaseLimit.Valid {
		value := uint(purchaseLimit.Int64)
		product.PurchaseLimit = &value
	}
	if badge.Valid {
		product.Badge = &badge.String
	}
	product.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	product.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return product, nil
}

func (h *Handler) get(ctx context.Context, id uint64) (Product, error) {
	return scanProduct(h.db.QueryRowContext(ctx, productColumns+` WHERE id = ?`, id))
}

func writeAudit(ctx context.Context, tx *sql.Tx, operatorID uint64, action, resourceID string, before, after any, r *http.Request) error {
	var beforeJSON, afterJSON any
	if before != nil {
		value, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = value
	}
	if after != nil {
		value, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = value
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (operator_user_id, action, resource_type, resource_id, before_snapshot, after_snapshot, request_ip, request_id)
		VALUES (?, ?, 'CARD_PRODUCT', ?, ?, ?, ?, ?)`, operatorID, action, resourceID, beforeJSON, afterJSON,
		strings.TrimSpace(r.Header.Get("X-Real-IP")), middleware.RequestIDFromContext(r.Context()))
	return err
}

var errVersionConflict = errors.New("card product version conflict")
