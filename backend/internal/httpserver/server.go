package httpserver

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/admincard"
	"urockclimbing.com/backend/internal/adminmember"
	"urockclimbing.com/backend/internal/adminops"
	"urockclimbing.com/backend/internal/adminorder"
	"urockclimbing.com/backend/internal/card"
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/respond"
)

type Dependencies struct {
	DB          *sql.DB
	Logger      *slog.Logger
	AppEnv      string
	StartedAt   time.Time
	PhoneCipher *securefield.Cipher
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	adminAuth := adminauth.NewHandler(deps.DB, deps.AppEnv)
	adminCards := admincard.NewHandler(deps.DB, adminAuth)
	adminMembers := adminmember.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	adminOrders := adminorder.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	adminOps := adminops.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	cardStore := card.NewHandler(deps.DB)
	mux.HandleFunc("POST /api/v1/admin/auth/login", adminAuth.Login)
	mux.HandleFunc("POST /api/v1/admin/auth/logout", adminAuth.Logout)
	mux.HandleFunc("POST /api/v1/admin/auth/change-password", adminAuth.ChangePassword)
	mux.HandleFunc("GET /api/v1/admin/me", adminAuth.Me)
	mux.HandleFunc("GET /api/v1/admin/card-products", adminCards.List)
	mux.HandleFunc("POST /api/v1/admin/card-products", adminCards.Create)
	mux.HandleFunc("PUT /api/v1/admin/card-products/{id}", adminCards.Update)
	mux.HandleFunc("POST /api/v1/admin/card-products/{id}/status", adminCards.ChangeStatus)
	mux.HandleFunc("GET /api/v1/admin/members", adminMembers.List)
	mux.HandleFunc("GET /api/v1/admin/members/{id}", adminMembers.Get)
	mux.HandleFunc("PUT /api/v1/admin/members/{id}", adminMembers.Update)
	mux.HandleFunc("GET /api/v1/admin/orders", adminOrders.List)
	mux.HandleFunc("GET /api/v1/admin/orders/{order_no}", adminOrders.Get)
	mux.HandleFunc("POST /api/v1/admin/orders/{order_no}/refund", adminOrders.Refund)
	mux.HandleFunc("GET /api/v1/admin/redemptions", adminOps.Redemptions)
	mux.HandleFunc("GET /api/v1/admin/staff", adminOps.Staff)
	mux.HandleFunc("PUT /api/v1/admin/users/{id}/role", adminOps.ChangeRole)
	mux.HandleFunc("GET /api/v1/admin/audit-logs", adminOps.AuditLogs)
	mux.HandleFunc("GET /api/v1/card-products", cardStore.ListOnSale)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		respond.JSON(w, r, http.StatusOK, map[string]any{
			"status":      "ok",
			"service":     "urock-api",
			"environment": deps.AppEnv,
			"started_at":  deps.StartedAt,
		})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := timeboxedContext(r, 2*time.Second)
		defer cancel()
		if err := deps.DB.PingContext(ctx); err != nil {
			respond.Error(w, r, http.StatusServiceUnavailable, "SERVICE_NOT_READY", "服务暂不可用")
			return
		}
		respond.JSON(w, r, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		respond.Error(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "接口不存在")
	})

	var handler http.Handler = mux
	handler = middleware.AccessLog(deps.Logger, handler)
	handler = middleware.Recovery(deps.Logger, handler)
	handler = middleware.RequestID(handler)
	return handler
}
