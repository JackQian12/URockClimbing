package adminauth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"urockclimbing.com/backend/internal/respond"
)

const (
	sessionCookieName = "urock_admin_session"
	sessionDuration   = 8 * time.Hour
	maxLoginAttempts  = 5
	lockDuration      = 15 * time.Minute
	bcryptCost        = 12
)

var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("invalid-password-placeholder"), bcryptCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

type Handler struct {
	db           *sql.DB
	secureCookie bool
}

type Session struct {
	SessionID          uint64
	UserID             uint64
	Username           string
	Nickname           sql.NullString
	MustChangePassword bool
	CSRFTokenHash      string
}

func NewHandler(db *sql.DB, appEnv string) *Handler {
	return &Handler{db: db, secureCookie: appEnv == "production"}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "请输入有效的用户名和密码")
		return
	}
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if input.Username == "" || input.Password == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	var credential struct {
		ID                 uint64
		UserID             uint64
		PasswordHash       string
		MustChangePassword bool
		FailedAttempts     uint
		LockedUntil        sql.NullTime
		Nickname           sql.NullString
		UserStatus         string
		Role               string
	}
	err := h.db.QueryRowContext(r.Context(), `
		SELECT c.id, c.user_id, c.password_hash, c.must_change_password,
		       c.failed_attempts, c.locked_until, u.nickname, u.status, u.role
		FROM admin_credentials c
		JOIN users u ON u.id = c.user_id
		WHERE c.username = ?`, input.Username).Scan(
		&credential.ID, &credential.UserID, &credential.PasswordHash,
		&credential.MustChangePassword, &credential.FailedAttempts,
		&credential.LockedUntil, &credential.Nickname, &credential.UserStatus, &credential.Role,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Run one password hash comparison to reduce username enumeration timing differences.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(input.Password))
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	if credential.LockedUntil.Valid && credential.LockedUntil.Time.After(time.Now().UTC()) {
		respond.Error(w, r, http.StatusTooManyRequests, "ADMIN_ACCOUNT_LOCKED", "登录失败次数过多，请稍后重试")
		return
	}
	if credential.UserStatus != "ACTIVE" || credential.Role != "ADMIN" || bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(input.Password)) != nil {
		attempts := credential.FailedAttempts + 1
		var lockedUntil any
		if attempts >= maxLoginAttempts {
			lockedUntil = time.Now().UTC().Add(lockDuration)
			attempts = 0
		}
		_, _ = h.db.ExecContext(r.Context(), `UPDATE admin_credentials SET failed_attempts = ?, locked_until = ? WHERE id = ?`, attempts, lockedUntil, credential.ID)
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	token, tokenHash, err := secureToken()
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	csrfToken, csrfHash, err := secureToken()
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	now := time.Now().UTC()
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE admin_credentials SET failed_attempts = 0, locked_until = NULL, last_login_at = ? WHERE id = ?`, now, credential.ID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO admin_sessions (user_id, token_hash, csrf_token_hash, expires_at, last_seen_at, request_ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?)`, credential.UserID, tokenHash, csrfHash, now.Add(sessionDuration), now, requestIP(r), truncate(r.UserAgent(), 255))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	h.setSessionCookie(w, token, now.Add(sessionDuration))
	respond.JSON(w, r, http.StatusOK, sessionResponse(credential.UserID, input.Username, credential.Nickname, credential.MustChangePassword, csrfToken))
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := h.Authenticate(r)
	if !ok {
		respond.Error(w, r, http.StatusUnauthorized, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return
	}
	if !h.ValidCSRF(session, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, http.StatusForbidden, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || len(input.NewPassword) < 12 || len(input.NewPassword) > 72 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "INVALID_PASSWORD", "新密码需为 12 至 72 个字符")
		return
	}
	if input.CurrentPassword == input.NewPassword {
		respond.Error(w, r, http.StatusUnprocessableEntity, "PASSWORD_UNCHANGED", "新密码不能与当前密码相同")
		return
	}
	var currentHash string
	if err := h.db.QueryRowContext(r.Context(), `SELECT password_hash FROM admin_credentials WHERE user_id = ?`, session.UserID).Scan(&currentHash); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.CurrentPassword)) != nil {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD", "当前密码错误")
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcryptCost)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	now := time.Now().UTC()
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE admin_credentials SET password_hash = ?, must_change_password = FALSE, password_changed_at = ?, failed_attempts = 0, locked_until = NULL WHERE user_id = ?`, string(newHash), now, session.UserID)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE admin_sessions SET revoked_at = ? WHERE user_id = ? AND id <> ? AND revoked_at IS NULL`, now, session.UserID, session.SessionID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	respond.JSON(w, r, http.StatusOK, map[string]bool{"password_changed": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := h.Authenticate(r)
	if !ok {
		respond.Error(w, r, http.StatusUnauthorized, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return
	}
	csrfToken, csrfHash, err := secureToken()
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	_, err = h.db.ExecContext(r.Context(), `UPDATE admin_sessions SET csrf_token_hash = ?, last_seen_at = ? WHERE id = ?`, csrfHash, time.Now().UTC(), session.SessionID)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
		return
	}
	respond.JSON(w, r, http.StatusOK, sessionResponse(session.UserID, session.Username, session.Nickname, session.MustChangePassword, csrfToken))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session, ok := h.Authenticate(r)
	if !ok {
		h.clearSessionCookie(w)
		respond.JSON(w, r, http.StatusOK, map[string]bool{"logged_out": true})
		return
	}
	if !h.ValidCSRF(session, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, http.StatusForbidden, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE admin_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, time.Now().UTC(), session.SessionID)
	h.clearSessionCookie(w)
	respond.JSON(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h *Handler) Authenticate(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return Session{}, false
	}
	var session Session
	err = h.db.QueryRowContext(r.Context(), `
		SELECT s.id, u.id, c.username, u.nickname, c.must_change_password, s.csrf_token_hash
		FROM admin_sessions s
		JOIN users u ON u.id = s.user_id
		JOIN admin_credentials c ON c.user_id = u.id
		WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?
		  AND u.status = 'ACTIVE' AND u.role = 'ADMIN'`, subtleHash(cookie.Value), time.Now().UTC()).Scan(
		&session.SessionID, &session.UserID, &session.Username, &session.Nickname,
		&session.MustChangePassword, &session.CSRFTokenHash,
	)
	return session, err == nil
}

func (h *Handler) ValidCSRF(session Session, token string) bool {
	return token != "" && subtleHash(token) == session.CSRFTokenHash
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: value, Path: "/api/v1/admin", Expires: expires, MaxAge: int(sessionDuration.Seconds()), HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Path: "/api/v1/admin", MaxAge: -1, HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteStrictMode})
}

func secureToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(value)
	return token, subtleHash(token), nil
}

func subtleHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func sessionResponse(userID uint64, username string, nickname sql.NullString, mustChange bool, csrfToken string) map[string]any {
	var displayName any
	if nickname.Valid {
		displayName = nickname.String
	}
	return map[string]any{"user": map[string]any{"id": strconv.FormatUint(userID, 10), "username": username, "nickname": displayName, "role": "ADMIN", "must_change_password": mustChange}, "csrf_token": csrfToken}
}

func requestIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return truncate(r.RemoteAddr, 64)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
