package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"urockclimbing.com/backend/internal/platform/wechat"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db     *sql.DB
	wechat wechat.LoginExchanger
	appID  string
	secret []byte
	now    func() time.Time
}

type CurrentUser struct {
	ID         uint64
	MemberNo   string
	Nickname   sql.NullString
	Avatar     sql.NullString
	Role       string
	Status     string
	Version    uint
	Registered bool
	PhoneLast4 sql.NullString
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func NewHandler(db *sql.DB, exchanger wechat.LoginExchanger, appID, secret string) *Handler {
	return &Handler{db: db, wechat: exchanger, appID: appID, secret: []byte(secret), now: time.Now}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.wechat == nil || h.appID == "" || len(h.secret) < 32 {
		respond.Error(w, r, http.StatusServiceUnavailable, "WECHAT_LOGIN_NOT_CONFIGURED", "微信登录暂未配置")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if decoder.Decode(&input) != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 256 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_LOGIN_CODE", "微信登录凭证无效")
		return
	}
	session, err := h.wechat.ExchangeCode(r.Context(), input.Code)
	if err != nil {
		respond.Error(w, r, http.StatusUnauthorized, "WECHAT_LOGIN_FAILED", "微信登录失败，请重试")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	defer tx.Rollback()
	user, err := findOrCreateUser(r, tx, h.appID, session)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	if user.Status != "ACTIVE" {
		respond.Error(w, r, http.StatusForbidden, "MEMBER_DISABLED", "会员账号已停用")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE users SET last_login_at = UTC_TIMESTAMP(3) WHERE id = ?`, user.ID); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	tokens, refreshID, err := h.issueTokens(r, tx, user.ID)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	_ = refreshID
	if err := tx.Commit(); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	respond.JSON(w, r, http.StatusOK, tokens)
}

func findOrCreateUser(r *http.Request, tx *sql.Tx, appID string, session wechat.Session) (CurrentUser, error) {
	var user CurrentUser
	err := tx.QueryRowContext(r.Context(), `
		SELECT u.id,u.member_no,u.nickname,u.avatar_url,u.role,u.status,u.version
		FROM wechat_identities wi JOIN users u ON u.id=wi.user_id
		WHERE wi.appid=? AND wi.openid=? FOR UPDATE`, appID, session.OpenID).
		Scan(&user.ID, &user.MemberNo, &user.Nickname, &user.Avatar, &user.Role, &user.Status, &user.Version)
	if err == nil {
		if session.UnionID != "" {
			_, _ = tx.ExecContext(r.Context(), `UPDATE wechat_identities SET unionid=COALESCE(unionid,?) WHERE appid=? AND openid=?`, session.UnionID, appID, session.OpenID)
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CurrentUser{}, err
	}
	memberNo, err := newMemberNo()
	if err != nil {
		return CurrentUser{}, err
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO users(member_no,role,status) VALUES(?,'MEMBER','ACTIVE')`, memberNo)
	if err != nil {
		return CurrentUser{}, err
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return CurrentUser{}, err
	}
	var unionID any
	if session.UnionID != "" {
		unionID = session.UnionID
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO wechat_identities(user_id,appid,openid,unionid) VALUES(?,?,?,?)`, userID, appID, session.OpenID, unionID); err != nil {
		return CurrentUser{}, err
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO member_profiles(user_id) VALUES(?)`, userID); err != nil {
		return CurrentUser{}, err
	}
	return CurrentUser{ID: uint64(userID), MemberNo: memberNo, Role: "MEMBER", Status: "ACTIVE", Version: 1}, nil
}

func newMemberNo() (string, error) {
	buffer := make([]byte, 5)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "UR" + time.Now().UTC().Format("2006") + strings.ToUpper(hex.EncodeToString(buffer)), nil
}

func (h *Handler) issueTokens(r *http.Request, tx *sql.Tx, userID uint64) (tokenResponse, int64, error) {
	now := h.now().UTC()
	access, err := issueAccessToken(h.secret, userID, now)
	if err != nil {
		return tokenResponse{}, 0, err
	}
	refresh, refreshHash, err := randomToken()
	if err != nil {
		return tokenResponse{}, 0, err
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES(?,?,?)`, userID, refreshHash, now.Add(refreshTokenTTL))
	if err != nil {
		return tokenResponse{}, 0, err
	}
	id, err := result.LastInsertId()
	return tokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(accessTokenTTL.Seconds())}, id, err
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || len(input.RefreshToken) < 32 || len(input.RefreshToken) > 256 {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "登录状态已失效，请重新登录")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "刷新登录状态失败")
		return
	}
	defer tx.Rollback()
	var oldID, userID uint64
	var expires time.Time
	var revoked sql.NullTime
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT rt.id,rt.user_id,rt.expires_at,rt.revoked_at,u.status FROM refresh_tokens rt JOIN users u ON u.id=rt.user_id WHERE rt.token_hash=? FOR UPDATE`, hashToken(input.RefreshToken)).Scan(&oldID, &userID, &expires, &revoked, &status)
	if err != nil || revoked.Valid || !expires.After(h.now().UTC()) || status != "ACTIVE" {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "登录状态已失效，请重新登录")
		return
	}
	tokens, newID, err := h.issueTokens(r, tx, userID)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "刷新登录状态失败")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE refresh_tokens SET revoked_at=UTC_TIMESTAMP(3),replaced_by_id=? WHERE id=? AND revoked_at IS NULL`, newID, oldID)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "刷新登录状态失败")
		return
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		respond.Error(w, r, 401, "INVALID_REFRESH_TOKEN", "登录状态已失效，请重新登录")
		return
	}
	if tx.Commit() != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "刷新登录状态失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.RefreshToken == "" {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REFRESH_TOKEN", "退出参数无效")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,UTC_TIMESTAMP(3)) WHERE token_hash=?`, hashToken(input.RefreshToken))
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "退出失败，请稍后重试")
		return
	}
	respond.JSON(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h *Handler) Authenticate(r *http.Request) (CurrentUser, error) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authorization, "Bearer ") || len(h.secret) < 32 {
		return CurrentUser{}, errors.New("missing bearer token")
	}
	userID, err := parseAccessToken(h.secret, strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), h.now().UTC())
	if err != nil {
		return CurrentUser{}, err
	}
	var user CurrentUser
	err = h.db.QueryRowContext(r.Context(), `
		SELECT u.id,u.member_no,u.nickname,u.avatar_url,u.role,u.status,u.version,
		       (SELECT p.phone_last4 FROM member_profiles p WHERE p.user_id=u.id),
		       EXISTS(SELECT 1 FROM member_profiles p WHERE p.user_id=u.id AND p.registered_at IS NOT NULL)
		FROM users u WHERE u.id=?`, userID).
		Scan(&user.ID, &user.MemberNo, &user.Nickname, &user.Avatar, &user.Role, &user.Status, &user.Version, &user.PhoneLast4, &user.Registered)
	if err != nil || user.Status != "ACTIVE" {
		return CurrentUser{}, errors.New("inactive or missing user")
	}
	return user, nil
}

func (h *Handler) RequireRegistered(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := h.Authenticate(r)
		if err != nil {
			respond.Error(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
			return
		}
		if !user.Registered {
			respond.Error(w, r, http.StatusForbidden, "REGISTRATION_REQUIRED", "请授权手机号完成会员注册")
			return
		}
		next(w, r)
	}
}

func (h *Handler) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := h.Authenticate(r); err != nil {
			respond.Error(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
			return
		}
		next(w, r)
	}
}

func ParseUserID(value string) (uint64, error) { return strconv.ParseUint(value, 10, 64) }
func FormatUserID(value uint64) string         { return strconv.FormatUint(value, 10) }
