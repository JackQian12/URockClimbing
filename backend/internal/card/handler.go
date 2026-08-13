package card

import (
	"database/sql"
	"net/http"
	"strconv"

	"urockclimbing.com/backend/internal/respond"
)

type Handler struct {
	db *sql.DB
}

type StoreProduct struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	ShortDescription string  `json:"short_description"`
	Description      string  `json:"description"`
	ProductType      string  `json:"product_type"`
	TotalTimes       *uint   `json:"total_times"`
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
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) ListOnSale(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, short_description, description, product_type, total_times, validity_days, activation_mode,
		       price_cent, list_price_cent, purchase_limit, daily_use_limit, transferable, rules, badge, theme_color
		FROM card_products WHERE status = 'ON_SALE' ORDER BY sort_order DESC, id DESC`)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员卡商品加载失败")
		return
	}
	defer rows.Close()
	products := make([]StoreProduct, 0)
	for rows.Next() {
		var product StoreProduct
		var id uint64
		var listPrice, purchaseLimit, totalTimes sql.NullInt64
		var badge sql.NullString
		if err := rows.Scan(&id, &product.Name, &product.ShortDescription, &product.Description,
			&product.ProductType, &totalTimes, &product.ValidityDays, &product.ActivationMode, &product.PriceCent,
			&listPrice, &purchaseLimit, &product.DailyUseLimit, &product.Transferable,
			&product.Rules, &badge, &product.ThemeColor); err != nil {
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员卡商品加载失败")
			return
		}
		product.ID = strconv.FormatUint(id, 10)
		if totalTimes.Valid {
			value := uint(totalTimes.Int64)
			product.TotalTimes = &value
		}
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
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "会员卡商品加载失败")
		return
	}
	respond.JSON(w, r, http.StatusOK, map[string]any{"items": products, "total": len(products)})
}
