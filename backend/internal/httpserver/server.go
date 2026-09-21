package httpserver

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/admincard"
	"urockclimbing.com/backend/internal/admincheckin"
	"urockclimbing.com/backend/internal/adminmember"
	"urockclimbing.com/backend/internal/adminops"
	"urockclimbing.com/backend/internal/adminorder"
	"urockclimbing.com/backend/internal/auth"
	"urockclimbing.com/backend/internal/card"
	"urockclimbing.com/backend/internal/member"
	"urockclimbing.com/backend/internal/membercard"
	"urockclimbing.com/backend/internal/middleware"
	"urockclimbing.com/backend/internal/order"
	"urockclimbing.com/backend/internal/payment"
	"urockclimbing.com/backend/internal/platform/securefield"
	"urockclimbing.com/backend/internal/platform/wechat"
	"urockclimbing.com/backend/internal/redemption"
	"urockclimbing.com/backend/internal/respond"
)

type Dependencies struct {
	DB                *sql.DB
	Logger            *slog.Logger
	AppEnv            string
	StartedAt         time.Time
	PhoneCipher       *securefield.Cipher
	AccessTokenSecret string
	WechatAppID       string
	WechatClient      wechat.LoginExchanger
	WechatPhoneClient wechat.PhoneNumberExchanger
	WechatPayGateway  wechat.PayRefundGateway
	WechatPayMchID    string
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	adminAuth := adminauth.NewHandler(deps.DB, deps.AppEnv)
	adminCards := admincard.NewHandler(deps.DB, adminAuth)
	adminCheckins := admincheckin.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	adminMembers := adminmember.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	adminOrders := adminorder.NewHandler(deps.DB, adminAuth, deps.PhoneCipher, deps.WechatPayGateway, deps.WechatPayMchID)
	adminOps := adminops.NewHandler(deps.DB, adminAuth, deps.PhoneCipher)
	cardStore := card.NewHandler(deps.DB)
	memberAuth := auth.NewHandler(deps.DB, deps.WechatClient, deps.WechatAppID, deps.AccessTokenSecret)
	members := member.NewHandler(deps.DB, memberAuth, deps.WechatPhoneClient, deps.PhoneCipher)
	memberCards := membercard.NewHandler(deps.DB, memberAuth)
	memberOrders := order.NewHandler(deps.DB, memberAuth)
	memberPayments := payment.NewHandler(deps.DB, memberAuth, deps.WechatPayGateway, deps.WechatAppID, deps.WechatPayMchID)
	redemptions := redemption.NewHandler(deps.DB, memberAuth)
	mux.HandleFunc("POST /api/v1/auth/wechat/login", memberAuth.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", memberAuth.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", memberAuth.Logout)
	mux.HandleFunc("GET /api/v1/me", members.Get)
	mux.HandleFunc("PUT /api/v1/me", members.Update)
	mux.HandleFunc("POST /api/v1/me/phone", members.RegisterPhone)
	mux.HandleFunc("GET /api/v1/me/cards", memberCards.List)
	mux.HandleFunc("GET /api/v1/me/cards/{id}", memberCards.Get)
	mux.HandleFunc("GET /api/v1/me/cards/{id}/redemptions", memberCards.CardRedemptions)
	mux.HandleFunc("GET /api/v1/me/redemptions", memberCards.Redemptions)
	mux.HandleFunc("POST /api/v1/me/cards/{id}/redemption-token", redemptions.CreateToken)
	mux.HandleFunc("POST /api/v1/me/checkins/preview", redemptions.CheckinPreview)
	mux.HandleFunc("POST /api/v1/me/checkins/confirm", redemptions.CheckinConfirm)
	mux.HandleFunc("POST /api/v1/staff/redemptions/preview", redemptions.Preview)
	mux.HandleFunc("POST /api/v1/staff/redemptions/confirm", redemptions.Confirm)
	mux.HandleFunc("GET /api/v1/staff/redemptions/today", redemptions.Today)
	mux.HandleFunc("POST /api/v1/admin/auth/login", adminAuth.Login)
	mux.HandleFunc("POST /api/v1/admin/auth/logout", adminAuth.Logout)
	mux.HandleFunc("POST /api/v1/admin/auth/change-password", adminAuth.ChangePassword)
	mux.HandleFunc("GET /api/v1/admin/me", adminAuth.Me)
	mux.HandleFunc("GET /api/v1/admin/card-products", adminCards.List)
	mux.HandleFunc("POST /api/v1/admin/card-products", adminCards.Create)
	mux.HandleFunc("PUT /api/v1/admin/card-products/{id}", adminCards.Update)
	mux.HandleFunc("POST /api/v1/admin/card-products/{id}/status", adminCards.ChangeStatus)
	mux.HandleFunc("GET /api/v1/admin/checkin-codes", adminCheckins.List)
	mux.HandleFunc("POST /api/v1/admin/checkin-codes", adminCheckins.Create)
	mux.HandleFunc("GET /api/v1/admin/checkin-codes/{id}", adminCheckins.Get)
	mux.HandleFunc("PUT /api/v1/admin/checkin-codes/{id}/status", adminCheckins.ChangeStatus)
	mux.HandleFunc("POST /api/v1/admin/checkin-codes/{id}/regenerate", adminCheckins.Regenerate)
	mux.HandleFunc("GET /api/v1/admin/members", adminMembers.List)
	mux.HandleFunc("GET /api/v1/admin/members/{id}", adminMembers.Get)
	mux.HandleFunc("PUT /api/v1/admin/members/{id}", adminMembers.Update)
	mux.HandleFunc("GET /api/v1/admin/orders", adminOrders.List)
	mux.HandleFunc("GET /api/v1/admin/orders/{order_no}", adminOrders.Get)
	mux.HandleFunc("POST /api/v1/admin/orders/{order_no}/refund", adminOrders.Refund)
	mux.HandleFunc("POST /api/v1/admin/refunds/{refund_no}/sync", adminOrders.SyncRefund)
	mux.HandleFunc("POST /api/v1/refunds/wechat/notify", adminOrders.RefundNotify)
	mux.HandleFunc("GET /api/v1/admin/redemptions", adminOps.Redemptions)
	mux.HandleFunc("GET /api/v1/admin/staff", adminOps.Staff)
	mux.HandleFunc("PUT /api/v1/admin/users/{id}/role", adminOps.ChangeRole)
	mux.HandleFunc("GET /api/v1/admin/audit-logs", adminOps.AuditLogs)
	mux.HandleFunc("GET /api/v1/card-products", memberAuth.RequireRegistered(cardStore.ListOnSale))
	mux.HandleFunc("GET /api/v1/card-products/{id}", memberAuth.RequireRegistered(cardStore.GetOnSale))
	mux.HandleFunc("POST /api/v1/orders", memberOrders.Create)
	mux.HandleFunc("GET /api/v1/me/orders", memberOrders.List)
	mux.HandleFunc("GET /api/v1/me/orders/{order_no}", memberOrders.Get)
	mux.HandleFunc("POST /api/v1/orders/{order_no}/wechat-pay", memberPayments.Prepare)
	mux.HandleFunc("POST /api/v1/orders/{order_no}/sync", memberPayments.Sync)
	mux.HandleFunc("POST /api/v1/payments/wechat/notify", memberPayments.Notify)
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
