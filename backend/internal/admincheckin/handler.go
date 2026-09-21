package admincheckin

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
	"unicode/utf8"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/respond"
)

const qrPrefix = "urock-checkin:"

var errVersionConflict = errors.New("check-in code version conflict")

type Handler struct {
	db     *sql.DB
	auth   *adminauth.Handler
	cipher *securefield.Cipher
}

type Code struct {
	ID        string `json:"id"`
	CodeNo    string `json:"code_no"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Version   uint   `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	QRPayload string `json:"qr_payload,omitempty"`
}

func NewHandler(db *sql.DB, auth *adminauth.Handler, cipher *securefield.Cipher) *Handler {
	return &Handler{db: db, auth: auth, cipher: cipher}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,code_no,name,status,version,created_at,updated_at FROM checkin_codes ORDER BY created_at DESC,id DESC`)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码列表加载失败")
		return
	}
	defer rows.Close()
	items := make([]Code, 0)
	for rows.Next() {
		item, err := scanCode(rows)
		if err != nil {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码列表加载失败")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码列表加载失败")
		return
	}
	respond.JSON(w, r, 200, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, false); !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	item, encrypted, err := h.get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, r, 404, "CHECKIN_CODE_NOT_FOUND", "签到码不存在")
		return
	}
	if err != nil || h.cipher == nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码加载失败")
		return
	}
	raw, err := h.cipher.Decrypt(encrypted)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码加载失败")
		return
	}
	item.QRPayload = qrPrefix + raw
	respond.JSON(w, r, 200, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if h.cipher == nil {
		respond.Error(w, r, 503, "CHECKIN_CODE_NOT_CONFIGURED", "签到码加密配置缺失")
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		respond.Error(w, r, 400, "INVALID_REQUEST", "请输入有效的签到码信息")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) < 2 || utf8.RuneCountInString(input.Name) > 100 {
		respond.Error(w, r, 422, "INVALID_CHECKIN_CODE", "签到码名称需为 2 至 100 个字符")
		return
	}
	raw, hash, encrypted, err := h.newToken()
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码创建失败")
		return
	}
	codeNo, err := businessNo()
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码创建失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `INSERT INTO checkin_codes(code_no,name,token_hash,token_encrypted,status,created_by) VALUES(?,?,?,?,'ACTIVE',?)`, codeNo, input.Name, hash, encrypted, session.UserID)
		if err == nil {
			var id int64
			id, err = result.LastInsertId()
			if err == nil {
				err = writeAudit(r.Context(), tx, session.UserID, "CHECKIN_CODE_CREATE", strconv.FormatInt(id, 10), nil, map[string]any{"code_no": codeNo, "name": input.Name, "status": "ACTIVE"}, r)
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码创建失败")
		return
	}
	item, err := scanCode(h.db.QueryRowContext(r.Context(), `SELECT id,code_no,name,status,version,created_at,updated_at FROM checkin_codes WHERE code_no=?`, codeNo))
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码已创建，但读取失败")
		return
	}
	item.QRPayload = qrPrefix + raw
	respond.JSON(w, r, 201, item)
}

func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input struct {
		Status  string `json:"status"`
		Version uint   `json:"version"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || (input.Status != "ACTIVE" && input.Status != "INACTIVE") || input.Version == 0 {
		respond.Error(w, r, 422, "INVALID_CHECKIN_CODE_STATUS", "请选择有效的签到码状态")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	var before Code
	if err == nil {
		before, err = scanCode(tx.QueryRowContext(r.Context(), `SELECT id,code_no,name,status,version,created_at,updated_at FROM checkin_codes WHERE id=? FOR UPDATE`, id))
	}
	if err == nil && before.Version != input.Version {
		err = errVersionConflict
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE checkin_codes SET status=?,version=version+1 WHERE id=? AND version=?`, input.Status, id, input.Version)
	}
	if err == nil {
		err = writeAudit(r.Context(), tx, session.UserID, "CHECKIN_CODE_STATUS_CHANGE", before.ID, before, map[string]any{"status": input.Status, "version": input.Version + 1}, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, r, 404, "CHECKIN_CODE_NOT_FOUND", "签到码不存在")
		} else if errors.Is(err, errVersionConflict) {
			respond.Error(w, r, 409, "VERSION_CONFLICT", "签到码状态已变化，请刷新后重试")
		} else {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码状态修改失败")
		}
		return
	}
	item, _, err := h.get(r.Context(), id)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码状态已修改，但读取失败")
		return
	}
	respond.JSON(w, r, 200, item)
}

func (h *Handler) Regenerate(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authorize(w, r, true)
	if !ok {
		return
	}
	if h.cipher == nil {
		respond.Error(w, r, 503, "CHECKIN_CODE_NOT_CONFIGURED", "签到码加密配置缺失")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input struct {
		Version uint `json:"version"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || input.Version == 0 {
		respond.Error(w, r, 422, "INVALID_CHECKIN_CODE", "签到码版本无效")
		return
	}
	raw, hash, encrypted, err := h.newToken()
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码更换失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	var before Code
	if err == nil {
		before, err = scanCode(tx.QueryRowContext(r.Context(), `SELECT id,code_no,name,status,version,created_at,updated_at FROM checkin_codes WHERE id=? FOR UPDATE`, id))
	}
	if err == nil && before.Version != input.Version {
		err = errVersionConflict
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE checkin_codes SET token_hash=?,token_encrypted=?,version=version+1 WHERE id=? AND version=?`, hash, encrypted, id, input.Version)
	}
	if err == nil {
		err = writeAudit(r.Context(), tx, session.UserID, "CHECKIN_CODE_REGENERATE", before.ID, before, map[string]any{"code_no": before.CodeNo, "name": before.Name, "status": before.Status, "version": input.Version + 1}, r)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, r, 404, "CHECKIN_CODE_NOT_FOUND", "签到码不存在")
		} else if errors.Is(err, errVersionConflict) {
			respond.Error(w, r, 409, "VERSION_CONFLICT", "签到码已变化，请刷新后重试")
		} else {
			respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码更换失败")
		}
		return
	}
	item, _, err := h.get(r.Context(), id)
	if err != nil {
		respond.Error(w, r, 500, "INTERNAL_ERROR", "签到码已更换，但读取失败")
		return
	}
	item.QRPayload = qrPrefix + raw
	respond.JSON(w, r, 200, item)
}

func (h *Handler) newToken() (string, string, []byte, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(random)
	sum := sha256.Sum256([]byte(raw))
	encrypted, err := h.cipher.Encrypt(raw)
	if err != nil {
		return "", "", nil, err
	}
	return raw, hex.EncodeToString(sum[:]), encrypted, nil
}

func (h *Handler) get(ctx context.Context, id uint64) (Code, []byte, error) {
	var item Code
	var rowID uint64
	var encrypted []byte
	var created, updated time.Time
	err := h.db.QueryRowContext(ctx, `SELECT id,code_no,name,status,version,created_at,updated_at,token_encrypted FROM checkin_codes WHERE id=?`, id).Scan(&rowID, &item.CodeNo, &item.Name, &item.Status, &item.Version, &created, &updated, &encrypted)
	if err != nil {
		return Code{}, nil, err
	}
	item.ID = strconv.FormatUint(rowID, 10)
	item.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	item.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return item, encrypted, nil
}

func scanCode(row interface{ Scan(...any) error }) (Code, error) {
	var item Code
	var id uint64
	var created, updated time.Time
	if err := row.Scan(&id, &item.CodeNo, &item.Name, &item.Status, &item.Version, &created, &updated); err != nil {
		return Code{}, err
	}
	item.ID = strconv.FormatUint(id, 10)
	item.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	item.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return item, nil
}

func parseID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	id, err := strconv.ParseUint(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id == 0 {
		respond.Error(w, r, 400, "INVALID_CHECKIN_CODE_ID", "签到码编号无效")
		return 0, false
	}
	return id, true
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, mutation bool) (adminauth.Session, bool) {
	session, ok := h.auth.Authenticate(r)
	if !ok {
		respond.Error(w, r, 401, "ADMIN_AUTH_REQUIRED", "请登录管理后台")
		return adminauth.Session{}, false
	}
	if session.MustChangePassword {
		respond.Error(w, r, 403, "PASSWORD_CHANGE_REQUIRED", "请先修改初始密码")
		return adminauth.Session{}, false
	}
	if mutation && !h.auth.ValidCSRF(session, r.Header.Get("X-CSRF-Token")) {
		respond.Error(w, r, 403, "CSRF_INVALID", "页面已过期，请刷新后重试")
		return adminauth.Session{}, false
	}
	return session, true
}

func writeAudit(ctx context.Context, tx *sql.Tx, operatorID uint64, action, resourceID string, before, after any, r *http.Request) error {
	encode := func(value any) (any, error) {
		if value == nil {
			return nil, nil
		}
		encoded, err := json.Marshal(value)
		return encoded, err
	}
	beforeJSON, err := encode(before)
	if err != nil {
		return err
	}
	afterJSON, err := encode(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(operator_user_id,action,resource_type,resource_id,before_snapshot,after_snapshot,request_ip,request_id) VALUES(?,?,'CHECKIN_CODE',?,?,?,?,?)`, operatorID, action, resourceID, beforeJSON, afterJSON, strings.TrimSpace(r.Header.Get("X-Real-IP")), middleware.RequestIDFromContext(r.Context()))
	return err
}

func businessNo() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "URCHK" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(random)), nil
}
