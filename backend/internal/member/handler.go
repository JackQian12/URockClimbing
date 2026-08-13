package member

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db   *sql.DB
	auth *auth.Handler
}

type profileResponse struct {
	ID        string  `json:"id"`
	MemberNo  string  `json:"member_no"`
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	Version   uint    `json:"version"`
}

func NewHandler(db *sql.DB, authenticator *auth.Handler) *Handler {
	return &Handler{db: db, auth: authenticator}
}

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

func profileFromUser(user auth.CurrentUser) profileResponse {
	profile := profileResponse{ID: auth.FormatUserID(user.ID), MemberNo: user.MemberNo, Role: user.Role, Status: user.Status, Version: user.Version}
	if user.Nickname.Valid {
		profile.Nickname = &user.Nickname.String
	}
	if user.Avatar.Valid {
		profile.AvatarURL = &user.Avatar.String
	}
	return profile
}
