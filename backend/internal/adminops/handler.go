package adminops

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
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db          *sql.DB
	auth        *adminauth.Handler
	phoneCipher *securefield.Cipher
}

func NewHandler(db *sql.DB, auth *adminauth.Handler, phoneCipher *securefield.Cipher) *Handler {
	return &Handler{db: db, auth: auth, phoneCipher: phoneCipher}
}

type Redemption struct {
	ID               string  `json:"id"`
	RedemptionNo     string  `json:"redemption_no"`
	Times            uint    `json:"times"`
	BeforeRemaining  *uint   `json:"before_remaining"`
	AfterRemaining   *uint   `json:"after_remaining"`
	RedeemedAt       string  `json:"redeemed_at"`
	RequestID        string  `json:"request_id"`
	MemberID         string  `json:"member_id"`
	MemberNo         string  `json:"member_no"`
	MemberNickname   *string `json:"member_nickname"`
	MemberPhone      *string `json:"member_phone"`
	CardNo           string  `json:"card_no"`
	ProductName      string  `json:"product_name"`
	CardStatus       string  `json:"card_status"`
	OperatorID       string  `json:"operator_id"`
	OperatorMemberNo string  `json:"operator_member_no"`
	OperatorNickname *string `json:"operator_nickname"`
}

type StaffUser struct {
	ID          string  `json:"id"`
	MemberNo    string  `json:"member_no"`
	Nickname    *string `json:"nickname"`
	AvatarURL   *string `json:"avatar_url"`
	Phone       *string `json:"phone"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	WechatBound bool    `json:"wechat_bound"`
	LastLoginAt *string `json:"last_login_at"`
	CreatedAt   string  `json:"created_at"`
	Version     uint    `json:"version"`
}

type AuditLog struct {
	ID           string          `json:"id"`
	OperatorID   *string         `json:"operator_id"`
	OperatorName *string         `json:"operator_name"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Before       json.RawMessage `json:"before"`
	After        json.RawMessage `json:"after"`
	RequestIP    *string         `json:"request_ip"`
	RequestID    string          `json:"request_id"`
	CreatedAt    string          `json:"created_at"`
}

func (h *Handler) Redemptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	page, pageSize := pagination(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	from, to, ok := dateRange(w, r)
	if !ok {
		return
	}
	where, args := redemptionWhere(q, from, to)
	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM redemption_records rr JOIN users u ON u.id=rr.user_id JOIN member_cards mc ON mc.id=rr.member_card_id JOIN users op ON op.id=rr.operator_user_id LEFT JOIN member_profiles mp ON mp.user_id=u.id `+where, args...).Scan(&total); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
		return
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(), redemptionSelect+where+` ORDER BY rr.redeemed_at DESC,rr.id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
		return
	}
	defer rows.Close()
	items := make([]Redemption, 0)
	for rows.Next() {
		item, e := scanRedemption(rows, h.phoneCipher)
		if e != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "核销记录加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) Staff(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	page, pageSize := pagination(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	role := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("role")))
	if role != "" && role != "MEMBER" && role != "STAFF" {
		respond.Error(w, r, 422, "INVALID_ROLE", "角色筛选无效")
		return
	}
	where, args := staffWhere(q, role)
	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users u LEFT JOIN member_profiles mp ON mp.user_id=u.id `+where, args...).Scan(&total); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "员工列表加载失败")
		return
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(), staffSelect+where+` ORDER BY (u.role='STAFF') DESC,u.created_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "员工列表加载失败")
		return
	}
	defer rows.Close()
	items := make([]StaffUser, 0)
	for rows.Next() {
		item, e := scanStaff(rows, h.phoneCipher)
		if e != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "员工列表加载失败")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "员工列表加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		respond.Error(w, r, 400, "INVALID_USER_ID", "用户编号无效")
		return
	}
	var input struct {
		Role    string `json:"role"`
		Version uint   `json:"version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || input.Version == 0 || (input.Role != "MEMBER" && input.Role != "STAFF") {
		respond.Error(w, r, 422, "INVALID_ROLE", "请选择有效角色")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	var beforeRole string
	var beforeVersion uint
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT u.role,u.version FROM users u JOIN member_profiles mp ON mp.user_id=u.id WHERE u.id=? AND u.role IN ('MEMBER','STAFF') AND mp.registered_at IS NOT NULL FOR UPDATE`, id).Scan(&beforeRole, &beforeVersion)
	}
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		respond.Error(w, r, 404, "USER_NOT_FOUND", "用户不存在或角色不可修改")
		return
	}
	if err == nil && beforeVersion != input.Version {
		err = errConflict
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE users SET role=?,version=version+1 WHERE id=?`, input.Role, id)
	}
	if err == nil {
		before, _ := json.Marshal(map[string]any{"role": beforeRole, "version": beforeVersion})
		after, _ := json.Marshal(input)
		_, err = tx.ExecContext(r.Context(), `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,before_snapshot,after_snapshot,request_ip,request_id) VALUES(?,'USER_ROLE_UPDATE','USER',?,?,?,?,?)`, session.UserID, strconv.FormatUint(id, 10), before, after, strings.TrimSpace(r.Header.Get("X-Real-IP")), middleware.RequestIDFromContext(r.Context()))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, errConflict) {
			respond.Error(w, r, 409, "VERSION_CONFLICT", "用户角色已被修改，请刷新后重试")
			return
		}
		respond.Error(w, r, 500, "INTERNAL_ERROR", "角色修改失败")
		return
	}
	item, e := h.getStaff(r.Context(), id)
	if e != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "角色已修改，但读取失败")
		return
	}
	respond.JSON(w, r, 200, item)
}

func (h *Handler) AuditLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	page, pageSize := pagination(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource_type"))
	from, to, ok := dateRange(w, r)
	if !ok {
		return
	}
	where, args := auditWhere(q, action, resource, from, to)
	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM audit_logs al LEFT JOIN users op ON op.id=al.operator_user_id `+where, args...).Scan(&total); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "审计日志加载失败")
		return
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(), auditSelect+where+` ORDER BY al.created_at DESC,al.id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "审计日志加载失败")
		return
	}
	defer rows.Close()
	items := make([]AuditLog, 0)
	for rows.Next() {
		item, e := scanAudit(rows)
		if e != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "审计日志加载失败")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "审计日志加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, mutation bool) (adminauth.Session, bool) {
	s, ok := h.auth.Authenticate(r)
	if !ok {
		respond.Error(w, r, 401, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return adminauth.Session{}, false
	}
	if s.MustChangePassword {
		respond.Error(w, r, 403, "PASSWORD_CHANGE_REQUIRED", "请先修改初始密码")
		return adminauth.Session{}, false
	}
	if mutation && !h.auth.ValidCSRF(s, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, 403, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return adminauth.Session{}, false
	}
	return s, true
}

const redemptionSelect = `SELECT rr.id,rr.redemption_no,rr.times,rr.before_remaining,rr.after_remaining,rr.redeemed_at,rr.request_id,u.id,u.member_no,u.nickname,u.phone_encrypted,mc.card_no,mc.product_name,mc.status,op.id,op.member_no,op.nickname FROM redemption_records rr JOIN users u ON u.id=rr.user_id JOIN member_cards mc ON mc.id=rr.member_card_id JOIN users op ON op.id=rr.operator_user_id LEFT JOIN member_profiles mp ON mp.user_id=u.id`

func redemptionWhere(q string, from, to *time.Time) (string, []any) {
	where := ` WHERE 1=1`
	args := []any{}
	if q != "" {
		like := "%" + escapeLike(q) + "%"
		where += ` AND (rr.redemption_no LIKE ? ESCAPE '!' OR u.member_no LIKE ? ESCAPE '!' OR u.nickname LIKE ? ESCAPE '!' OR mc.card_no LIKE ? ESCAPE '!' OR op.nickname LIKE ? ESCAPE '!' OR mp.phone_last4=?)`
		args = append(args, like, like, like, like, like, q)
	}
	if from != nil {
		where += ` AND rr.redeemed_at>=?`
		args = append(args, *from)
	}
	if to != nil {
		where += ` AND rr.redeemed_at<?`
		args = append(args, *to)
	}
	return where, args
}
func scanRedemption(row interface{ Scan(...any) error }, c *securefield.Cipher) (Redemption, error) {
	var v Redemption
	var id, uid, oid uint64
	var nick, onick sql.NullString
	var phone []byte
	var redeemed time.Time
	var before, after sql.NullInt64
	if err := row.Scan(&id, &v.RedemptionNo, &v.Times, &before, &after, &redeemed, &v.RequestID, &uid, &v.MemberNo, &nick, &phone, &v.CardNo, &v.ProductName, &v.CardStatus, &oid, &v.OperatorMemberNo, &onick); err != nil {
		return v, err
	}
	if before.Valid {
		value := uint(before.Int64)
		v.BeforeRemaining = &value
	}
	if after.Valid {
		value := uint(after.Int64)
		v.AfterRemaining = &value
	}
	v.ID = strconv.FormatUint(id, 10)
	v.MemberID = strconv.FormatUint(uid, 10)
	v.OperatorID = strconv.FormatUint(oid, 10)
	v.MemberNickname = nullString(nick)
	v.OperatorNickname = nullString(onick)
	v.RedeemedAt = formatTime(redeemed)
	if len(phone) > 0 {
		if c == nil {
			return v, errors.New("phone cipher missing")
		}
		p, e := c.Decrypt(phone)
		if e != nil {
			return v, e
		}
		v.MemberPhone = &p
	}
	return v, nil
}

const staffSelect = `SELECT u.id,u.member_no,u.nickname,u.avatar_url,u.phone_encrypted,u.role,u.status,EXISTS(SELECT 1 FROM wechat_identities wi WHERE wi.user_id=u.id),u.last_login_at,u.created_at,u.version FROM users u LEFT JOIN member_profiles mp ON mp.user_id=u.id`

func staffWhere(q, role string) (string, []any) {
	where := ` WHERE u.role IN ('MEMBER','STAFF') AND mp.registered_at IS NOT NULL`
	args := []any{}
	if q != "" {
		like := "%" + escapeLike(q) + "%"
		where += ` AND (u.member_no LIKE ? ESCAPE '!' OR u.nickname LIKE ? ESCAPE '!' OR mp.phone_last4=?)`
		args = append(args, like, like, q)
	}
	if role != "" {
		where += ` AND u.role=?`
		args = append(args, role)
	}
	return where, args
}
func scanStaff(row interface{ Scan(...any) error }, c *securefield.Cipher) (StaffUser, error) {
	var v StaffUser
	var id uint64
	var nick, avatar sql.NullString
	var phone []byte
	var last sql.NullTime
	var created time.Time
	if err := row.Scan(&id, &v.MemberNo, &nick, &avatar, &phone, &v.Role, &v.Status, &v.WechatBound, &last, &created, &v.Version); err != nil {
		return v, err
	}
	v.ID = strconv.FormatUint(id, 10)
	v.Nickname = nullString(nick)
	v.AvatarURL = nullString(avatar)
	v.LastLoginAt = timeString(last)
	v.CreatedAt = formatTime(created)
	if len(phone) > 0 {
		if c == nil {
			return v, errors.New("phone cipher missing")
		}
		p, e := c.Decrypt(phone)
		if e != nil {
			return v, e
		}
		v.Phone = &p
	}
	return v, nil
}
func (h *Handler) getStaff(ctx context.Context, id uint64) (StaffUser, error) {
	return scanStaff(h.db.QueryRowContext(ctx, staffSelect+` WHERE u.id=? AND u.role IN ('MEMBER','STAFF') AND mp.registered_at IS NOT NULL`, id), h.phoneCipher)
}

const auditSelect = `SELECT al.id,al.operator_user_id,COALESCE(op.nickname,op.member_no),al.action,al.resource_type,al.resource_id,al.before_snapshot,al.after_snapshot,al.request_ip,al.request_id,al.created_at FROM audit_logs al LEFT JOIN users op ON op.id=al.operator_user_id`

func auditWhere(q, action, resource string, from, to *time.Time) (string, []any) {
	where := ` WHERE 1=1`
	args := []any{}
	if q != "" {
		like := "%" + escapeLike(q) + "%"
		where += ` AND (al.resource_id LIKE ? ESCAPE '!' OR al.request_id LIKE ? ESCAPE '!' OR op.nickname LIKE ? ESCAPE '!')`
		args = append(args, like, like, like)
	}
	if action != "" {
		where += ` AND al.action=?`
		args = append(args, action)
	}
	if resource != "" {
		where += ` AND al.resource_type=?`
		args = append(args, resource)
	}
	if from != nil {
		where += ` AND al.created_at>=?`
		args = append(args, *from)
	}
	if to != nil {
		where += ` AND al.created_at<?`
		args = append(args, *to)
	}
	return where, args
}
func scanAudit(row interface{ Scan(...any) error }) (AuditLog, error) {
	var v AuditLog
	var id uint64
	var operator sql.NullInt64
	var name, ip sql.NullString
	var before, after []byte
	var created time.Time
	if err := row.Scan(&id, &operator, &name, &v.Action, &v.ResourceType, &v.ResourceID, &before, &after, &ip, &v.RequestID, &created); err != nil {
		return v, err
	}
	v.ID = strconv.FormatUint(id, 10)
	if operator.Valid {
		s := strconv.FormatInt(operator.Int64, 10)
		v.OperatorID = &s
	}
	v.OperatorName = nullString(name)
	v.RequestIP = nullString(ip)
	if len(before) > 0 {
		v.Before = json.RawMessage(before)
	}
	if len(after) > 0 {
		v.After = json.RawMessage(after)
	}
	v.CreatedAt = formatTime(created)
	return v, nil
}

func pagination(r *http.Request) (int, int) {
	return boundedInt(r.URL.Query().Get("page"), 1, 1, 100000), boundedInt(r.URL.Query().Get("page_size"), 20, 1, 50)
}
func dateRange(w http.ResponseWriter, r *http.Request) (*time.Time, *time.Time, bool) {
	parse := func(raw string) (*time.Time, error) {
		if raw == "" {
			return nil, nil
		}
		v, e := time.Parse("2006-01-02", raw)
		return &v, e
	}
	from, e := parse(r.URL.Query().Get("date_from"))
	if e != nil {
		respond.Error(w, r, 422, "INVALID_DATE", "开始日期无效")
		return nil, nil, false
	}
	to, e := parse(r.URL.Query().Get("date_to"))
	if e != nil {
		respond.Error(w, r, 422, "INVALID_DATE", "结束日期无效")
		return nil, nil, false
	}
	if to != nil {
		next := to.Add(24 * time.Hour)
		to = &next
	}
	return from, to, true
}
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
func escapeLike(v string) string {
	v = strings.ReplaceAll(v, "!", "!!")
	v = strings.ReplaceAll(v, "%", "!%")
	return strings.ReplaceAll(v, "_", "!_")
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

var errConflict = errors.New("version conflict")
