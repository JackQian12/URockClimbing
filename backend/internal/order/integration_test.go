//go:build integration

package order

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func integrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Fatal("MYSQL_TEST_DSN is required for integration tests")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var databaseName string
	if err := db.QueryRow(`SELECT DATABASE()`).Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Fatalf("refusing to modify non-test database %q", databaseName)
	}
	for _, table := range []string{"redemption_records", "redemption_tokens", "refund_transactions", "member_cards", "payment_transactions", "orders", "admin_sessions", "admin_credentials", "audit_logs", "refresh_tokens", "wechat_identities", "member_profiles", "card_products", "users"} {
		if _, err := db.Exec(`DELETE FROM ` + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	return db
}

func seedOrderFixture(t *testing.T, db *sql.DB, status string, purchaseLimit any) (uint64, uint64) {
	t.Helper()
	userResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URTESTORDER','MEMBER','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	if _, err := db.Exec(`INSERT INTO member_profiles(user_id,registered_at) VALUES(?,UTC_TIMESTAMP(3))`, userID); err != nil {
		t.Fatal(err)
	}
	productResult, err := db.Exec(`INSERT INTO card_products(name,short_description,description,product_type,total_times,validity_days,activation_mode,price_cent,list_price_cent,purchase_limit,daily_use_limit,transferable,rules,badge,theme_color,status,sort_order,version) VALUES('十次卡','测试商品','原始说明','COUNT_CARD',10,180,'FIRST_USE',88000,NULL,?,1,FALSE,'测试规则',NULL,'#5A2D16',?,0,1)`, purchaseLimit, status)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ := productResult.LastInsertId()
	return uint64(userID), uint64(productID)
}

func TestCreateOrderIsIdempotentAndKeepsSnapshot(t *testing.T) {
	db := integrationDB(t)
	userID, productID := seedOrderFixture(t, db, "ON_SALE", nil)
	handler := &Handler{db: db, now: func() time.Time { return time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC) }}

	first, created, err := handler.create(context.Background(), userID, productID, "same-request-key")
	if err != nil || !created {
		t.Fatalf("first create: created=%v err=%v", created, err)
	}
	second, created, err := handler.create(context.Background(), userID, productID, "same-request-key")
	if err != nil || created || second.OrderNo != first.OrderNo {
		t.Fatalf("idempotent create: order=%q created=%v err=%v", second.OrderNo, created, err)
	}
	if _, err := db.Exec(`UPDATE card_products SET name='改名后',price_cent=99000 WHERE id=?`, productID); err != nil {
		t.Fatal(err)
	}
	stored, err := getOrder(context.Background(), db, userID, first.OrderNo, "")
	if err != nil {
		t.Fatal(err)
	}
	if stored.TotalAmountCent != 88000 || stored.ProductSnapshot.Name != "十次卡" || stored.ProductSnapshot.PriceCent != 88000 {
		t.Fatalf("snapshot changed with product: %#v", stored)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id=?`, userID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("order count=%d err=%v", count, err)
	}
}

func TestConcurrentCreateReturnsOneOrder(t *testing.T) {
	db := integrationDB(t)
	userID, productID := seedOrderFixture(t, db, "ON_SALE", nil)
	handler := &Handler{db: db, now: time.Now}
	start := make(chan struct{})
	orders := make([]Order, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range orders {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			<-start
			orders[i], _, errs[i] = handler.create(context.Background(), userID, productID, "concurrent-request")
		}(index)
	}
	close(start)
	wait.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("concurrent errors: %v %v", errs[0], errs[1])
	}
	if orders[0].OrderNo == "" || orders[0].OrderNo != orders[1].OrderNo {
		t.Fatalf("different orders returned: %q %q", orders[0].OrderNo, orders[1].OrderNo)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM orders`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected one order, got %d", count)
	}
}

func TestCreateRejectsOffSaleAndPurchaseLimit(t *testing.T) {
	db := integrationDB(t)
	userID, productID := seedOrderFixture(t, db, "OFF_SALE", 1)
	handler := &Handler{db: db, now: time.Now}
	if _, _, err := handler.create(context.Background(), userID, productID, "offsale-request"); !errors.Is(err, errProductNotFound) {
		t.Fatalf("expected off-sale error, got %v", err)
	}
	if _, err := db.Exec(`UPDATE card_products SET status='ON_SALE' WHERE id=?`, productID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := handler.create(context.Background(), userID, productID, "first-limited"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := handler.create(context.Background(), userID, productID, "second-limited"); !errors.Is(err, errPurchaseLimit) {
		t.Fatalf("expected purchase limit error, got %v", err)
	}
}
