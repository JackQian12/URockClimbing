package member

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/platform/wechat"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db          *sql.DB
	auth        *auth.Handler
	phones      wechat.PhoneNumberExchanger
	phoneCipher *securefield.Cipher
}

type profileResponse struct {
	ID          string  `json:"id"`
	MemberNo    string  `json:"member_no"`
	Nickname    *string `json:"nickname"`
	AvatarURL   *string `json:"avatar_url"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	Version     uint    `json:"version"`
	Registered  bool    `json:"registered"`
	PhoneMasked *string `json:"phone_masked"`
}

func NewHandler(db *sql.DB, authenticator *auth.Handler, phones wechat.PhoneNumberExchanger, phoneCipher *securefield.Cipher) *Handler {
	return &Handler{db: db, auth: authenticator, phones: phones, phoneCipher: phoneCipher}
}

var phonePattern = regexp.MustCompile(`^[0-9]{6,20}$`)

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
		return
	}
	respond.JSON(w, r, http.StatusOK, profileFromUser(user))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
		return
	}
	var input struct {
		Nickname string `json:"nickname"`
		Version  uint   `json:"version"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "会员资料格式无效")
		return
	}
	input.Nickname = strings.TrimSpace(input.Nickname)
	if input.Version == 0 || utf8.RuneCountInString(input.Nickname) > 64 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "昵称最多 64 个字符")
		return
	}
	var nickname any
	if input.Nickname != "" {
		nickname = input.Nickname
	}
	result, err := h.db.ExecContext(r.Context(), `UPDATE users SET nickname=?,version=version+1 WHERE id=? AND version=?`, nickname, user.ID, input.Version)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员资料保存失败")
		return
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		respond.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "会员资料已更新，请刷新后重试")
		return
	}
	updated, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员资料读取失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, profileFromUser(updated))
}

func (h *Handler) RegisterPhone(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "请先登录")
		return
	}
	if user.Registered {
		respond.JSON(w, r, http.StatusOK, profileFromUser(user))
		return
	}
	if h.phones == nil || h.phoneCipher == nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "PHONE_REGISTRATION_NOT_CONFIGURED", "手机号注册暂未配置")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || strings.TrimSpace(input.Code) == "" || len(input.Code) > 512 {
		respond.Error(w, r, http.StatusBadRequest, "INVALID_PHONE_CODE", "手机号授权凭证无效")
		return
	}
	phone, err := h.phones.ExchangePhoneNumber(r.Context(), input.Code)
	if err != nil || !phonePattern.MatchString(phone.PureNumber) {
		respond.Error(w, r, http.StatusBadGateway, "WECHAT_PHONE_FAILED", "手机号授权失败，请重试")
		return
	}
	encrypted, err := h.phoneCipher.Encrypt(phone.PureNumber)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员注册失败")
		return
	}
	last4 := phone.PureNumber
	if len(last4) > 4 {
		last4 = last4[len(last4)-4:]
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员注册失败")
		return
	}
	defer tx.Rollback()
	var alreadyRegistered bool
	err = tx.QueryRowContext(r.Context(), `SELECT registered_at IS NOT NULL FROM member_profiles WHERE user_id=? FOR UPDATE`, user.ID).Scan(&alreadyRegistered)
	if err == nil && !alreadyRegistered {
		_, err = tx.ExecContext(r.Context(), `UPDATE users SET phone_encrypted=? WHERE id=?`, encrypted, user.ID)
	}
	if err == nil && !alreadyRegistered {
		_, err = tx.ExecContext(r.Context(), `UPDATE member_profiles SET phone_last4=?,phone_authorized_at=UTC_TIMESTAMP(3),registered_at=UTC_TIMESTAMP(3) WHERE user_id=?`, last4, user.ID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员注册失败")
		return
	}
	updated, err := h.auth.Authenticate(r)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员资料读取失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, profileFromUser(updated))
}

func profileFromUser(user auth.CurrentUser) profileResponse {
	profile := profileResponse{ID: auth.FormatUserID(user.ID), MemberNo: user.MemberNo, Role: user.Role, Status: user.Status, Version: user.Version, Registered: user.Registered}
	if user.Nickname.Valid {
		profile.Nickname = &user.Nickname.String
	}
	if user.Avatar.Valid {
		profile.AvatarURL = &user.Avatar.String
	}
	if user.PhoneLast4.Valid {
		masked := "*******" + user.PhoneLast4.String
		profile.PhoneMasked = &masked
	}
	return profile
}
