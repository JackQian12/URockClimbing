package adminmember

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

type Member struct {
	ID                    string   `json:"id"`
	MemberNo              string   `json:"member_no"`
	Nickname              *string  `json:"nickname"`
	AvatarURL             *string  `json:"avatar_url"`
	Phone                 *string  `json:"phone"`
	PhoneAuthorized       bool     `json:"phone_authorized"`
	WechatBound           bool     `json:"wechat_bound"`
	Status                string   `json:"status"`
	Role                  string   `json:"role"`
	Tags                  []string `json:"tags"`
	AdminNote             string   `json:"admin_note"`
	ActiveCardCount       uint     `json:"active_card_count"`
	RemainingTimes        uint     `json:"remaining_times"`
	PaidOrderCount        uint     `json:"paid_order_count"`
	TotalSpentCent        uint64   `json:"total_spent_cent"`
	RedemptionCount       uint     `json:"redemption_count"`
	LastRedemptionAt      *string  `json:"last_redemption_at"`
	LastLoginAt           *string  `json:"last_login_at"`
	RegisteredAt          string   `json:"registered_at"`
	PrivacyConsentVersion *string  `json:"privacy_consent_version"`
	PrivacyConsentAt      *string  `json:"privacy_consent_at"`
	MarketingConsent      bool     `json:"marketing_consent"`
	Version               uint     `json:"version"`
}

type updateInput struct {
	Status    string   `json:"status"`
	Tags      []string `json:"tags"`
	AdminNote string   `json:"admin_note"`
	Version   uint     `json:"version"`
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
	if status != "" && status != "ACTIVE" && status != "DISABLED" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_MEMBER_STATUS", "会员状态筛选无效")
		return
	}
	page := boundedInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := boundedInt(r.URL.Query().Get("page_size"), 20, 1, 50)
	where, args := memberWhere(query, status)

	var total int
	if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users u LEFT JOIN member_profiles p ON p.user_id = u.id `+where, args...).Scan(&total); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员列表加载失败")
		return
	}
	listArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(), memberSelect+where+` ORDER BY u.created_at DESC, u.id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员列表加载失败")
		return
	}
	defer rows.Close()
	members := make([]Member, 0)
	for rows.Next() {
		member, err := scanMember(rows, h.phoneCipher)
		if err != nil {
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员列表加载失败")
			return
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员列表加载失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, map[string]any{
		"items": members, "total": total, "page": page, "page_size": pageSize,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	member, err := h.get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "MEMBER_NOT_FOUND", "会员不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员详情加载失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, member)
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
	var input updateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "请输入有效的会员信息")
		return
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	input.AdminNote = strings.TrimSpace(input.AdminNote)
	input.Tags = normalizeTags(input.Tags)
	if message := validateUpdate(input); message != "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_MEMBER", message)
		return
	}
	before, err := h.get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "MEMBER_NOT_FOUND", "会员不存在")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员详情加载失败")
		return
	}
	tagsJSON, _ := json.Marshal(input.Tags)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `UPDATE users SET status = ?, version = version + 1 WHERE id = ? AND role = 'MEMBER' AND version = ?`, input.Status, id, input.Version)
		if err == nil {
			var affected int64
			affected, err = result.RowsAffected()
			if err == nil && affected != 1 {
				err = errVersionConflict
			}
		}
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO member_profiles (user_id, tags, admin_note) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE tags = VALUES(tags), admin_note = VALUES(admin_note)`, id, tagsJSON, input.AdminNote)
	}
	if err == nil {
		err = writeAudit(r.Context(), tx, session.UserID, strconv.FormatUint(id, 10), before, input, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, errVersionConflict) {
			respond.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "会员资料已被其他管理员修改，请刷新后重试")
			return
		}
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员资料保存失败")
		return
	}
	member, err := h.get(r.Context(), id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员资料保存成功，但读取失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, member)
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

func memberWhere(query, status string) (string, []any) {
	where := ` WHERE u.role = 'MEMBER'`
	args := make([]any, 0, 4)
	if query != "" {
		like := "%" + escapeLike(query) + "%"
		where += ` AND (u.member_no LIKE ? ESCAPE '!' OR u.nickname LIKE ? ESCAPE '!' OR p.phone_last4 = ?)`
		args = append(args, like, like, query)
	}
	if status != "" {
		where += ` AND u.status = ?`
		args = append(args, status)
	}
	return where, args
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	return strings.ReplaceAll(value, "_", "!_")
}

const memberSelect = `
	SELECT u.id, u.member_no, u.nickname, u.avatar_url, u.phone_encrypted, p.phone_last4,
	       EXISTS(SELECT 1 FROM wechat_identities wi WHERE wi.user_id = u.id),
	       u.status, u.role, COALESCE(p.tags, JSON_ARRAY()), COALESCE(p.admin_note, ''),
	       (SELECT COUNT(*) FROM member_cards mc WHERE mc.user_id = u.id AND mc.status = 'ACTIVE' AND mc.expires_at > UTC_TIMESTAMP(3)),
	       COALESCE((SELECT SUM(mc.remaining_times) FROM member_cards mc WHERE mc.user_id = u.id AND mc.status = 'ACTIVE' AND mc.expires_at > UTC_TIMESTAMP(3)), 0),
	       (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id AND o.status IN ('PAID', 'REFUNDING')),
	       COALESCE((SELECT SUM(o.paid_amount_cent) FROM orders o WHERE o.user_id = u.id AND o.status IN ('PAID', 'REFUNDING')), 0),
	       (SELECT COUNT(*) FROM redemption_records rr WHERE rr.user_id = u.id),
	       (SELECT MAX(rr.redeemed_at) FROM redemption_records rr WHERE rr.user_id = u.id),
	       u.last_login_at, u.created_at, p.privacy_consent_version, p.privacy_consent_at,
	       (p.marketing_consent_at IS NOT NULL), u.version
	FROM users u LEFT JOIN member_profiles p ON p.user_id = u.id`

type scanner interface {
	Scan(dest ...any) error
}

func scanMember(row scanner, phoneCipher *securefield.Cipher) (Member, error) {
	var member Member
	var id uint64
	var nickname, avatar, phoneLast4, privacyVersion sql.NullString
	var phoneEncrypted []byte
	var lastRedemption, lastLogin, privacyConsent sql.NullTime
	var registeredAt time.Time
	var tagsJSON []byte
	err := row.Scan(&id, &member.MemberNo, &nickname, &avatar, &phoneEncrypted, &phoneLast4, &member.WechatBound,
		&member.Status, &member.Role, &tagsJSON, &member.AdminNote, &member.ActiveCardCount,
		&member.RemainingTimes, &member.PaidOrderCount, &member.TotalSpentCent, &member.RedemptionCount,
		&lastRedemption, &lastLogin, &registeredAt, &privacyVersion, &privacyConsent,
		&member.MarketingConsent, &member.Version)
	if err != nil {
		return Member{}, err
	}
	member.ID = strconv.FormatUint(id, 10)
	if nickname.Valid {
		member.Nickname = &nickname.String
	}
	if avatar.Valid {
		member.AvatarURL = &avatar.String
	}
	if len(phoneEncrypted) > 0 {
		if phoneCipher == nil {
			return Member{}, errors.New("phone cipher is not configured")
		}
		phone, decryptErr := phoneCipher.Decrypt(phoneEncrypted)
		if decryptErr != nil {
			return Member{}, decryptErr
		}
		member.Phone = &phone
		member.PhoneAuthorized = true
	} else if phoneLast4.Valid {
		member.PhoneAuthorized = true
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &member.Tags)
	}
	if member.Tags == nil {
		member.Tags = []string{}
	}
	member.LastRedemptionAt = timePointer(lastRedemption)
	member.LastLoginAt = timePointer(lastLogin)
	member.RegisteredAt = registeredAt.UTC().Format(time.RFC3339Nano)
	if privacyVersion.Valid {
		member.PrivacyConsentVersion = &privacyVersion.String
	}
	member.PrivacyConsentAt = timePointer(privacyConsent)
	return member, nil
}

func timePointer(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func (h *Handler) get(ctx context.Context, id uint64) (Member, error) {
	return scanMember(h.db.QueryRowContext(ctx, memberSelect+` WHERE u.id = ? AND u.role = 'MEMBER'`, id), h.phoneCipher)
}

func pathID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_MEMBER_ID", "会员编号无效")
		return 0, false
	}
	return id, true
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

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	return result
}

func validateUpdate(input updateInput) string {
	if input.Status != "ACTIVE" && input.Status != "DISABLED" {
		return "请选择有效的会员状态"
	}
	if input.Version == 0 {
		return "会员版本无效，请刷新后重试"
	}
	if utf8.RuneCountInString(input.AdminNote) > 500 {
		return "内部备注不能超过 500 个字符"
	}
	if len(input.Tags) > 10 {
		return "每位会员最多设置 10 个标签"
	}
	for _, tag := range input.Tags {
		if utf8.RuneCountInString(tag) > 20 {
			return "单个会员标签不能超过 20 个字符"
		}
	}
	return ""
}

func writeAudit(ctx context.Context, tx *sql.Tx, operatorID uint64, resourceID string, before Member, after updateInput, r *http.Request) error {
	before.Phone = nil
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (operator_user_id, action, resource_type, resource_id, before_snapshot, after_snapshot, request_ip, request_id)
		VALUES (?, 'MEMBER_UPDATE', 'MEMBER', ?, ?, ?, ?, ?)`, operatorID, resourceID, beforeJSON, afterJSON,
		strings.TrimSpace(r.Header.Get("X-Real-IP")), middleware.RequestIDFromContext(r.Context()))
	return err
}

var errVersionConflict = errors.New("member version conflict")
