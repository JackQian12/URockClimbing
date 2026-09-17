//go:build integration

package redemption

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func redemptionIntegrationDB(t *testing.T) *sql.DB {
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

func seedCard(t *testing.T, db *sql.DB, productType, status string) (uint64, uint64, uint64) {
	t.Helper()
	memberResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URMEMBERREDEEM','MEMBER','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	memberID, _ := memberResult.LastInsertId()
	staffResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URSTAFFREDEEM','STAFF','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	staffID, _ := staffResult.LastInsertId()
	var total any = 3
	if productType == "TIME_PASS" {
		total = nil
	}
	productResult, err := db.Exec(`INSERT INTO card_products(name,short_description,description,product_type,total_times,validity_days,activation_mode,price_cent,daily_use_limit,transferable,rules,status,sort_order,version) VALUES('核销测试卡','测试','测试',?,?,30,'FIRST_USE',100,1,FALSE,'测试','ON_SALE',0,1)`, productType, total)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ := productResult.LastInsertId()
	orderResult, err := db.Exec(`INSERT INTO orders(order_no,user_id,status,total_amount_cent,paid_amount_cent,product_snapshot,client_request_id,expires_at,paid_at) VALUES('URORDREDEEMTEST',?,'PAID',100,100,JSON_OBJECT('name','核销测试卡'),'redeem-order',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, memberID)
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := orderResult.LastInsertId()
	var activated, expires any
	if status == "ACTIVE" {
		activated = time.Now().UTC()
		expires = time.Now().UTC().Add(30 * 24 * time.Hour)
	}
	cardResult, err := db.Exec(`INSERT INTO member_cards(card_no,user_id,source_order_id,product_id,product_name,product_type,total_times,remaining_times,status,activated_at,expires_at) VALUES('URCARDREDEEMTEST',?,?,?,'核销测试卡',?,?,?,?,?,?)`, memberID, orderID, productID, productType, total, total, status, activated, expires)
	if err != nil {
		t.Fatal(err)
	}
	cardID, _ := cardResult.LastInsertId()
	return uint64(memberID), uint64(staffID), uint64(cardID)
}

func TestConcurrentConfirmConsumesTokenOnce(t *testing.T) {
	db := redemptionIntegrationDB(t)
	memberID, staffID, cardID := seedCard(t, db, "COUNT_CARD", "PENDING_ACTIVATION")
	now := time.Now().UTC().Truncate(time.Millisecond)
	h := &Handler{db: db, now: func() time.Time { return now }}
	token, err := h.createToken(context.Background(), memberID, cardID)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for i := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			req := httptest.NewRequest("POST", "/staff/redemptions/confirm", nil)
			_, errs[index] = h.confirm(context.Background(), staffID, token.Token, "redeem-concurrent-"+string(rune('a'+index)), req)
		}(i)
	}
	close(start)
	wait.Wait()
	successes, used := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, errTokenAlreadyUsed) {
			used++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || used != 1 {
		t.Fatalf("successes=%d already_used=%d errors=%v", successes, used, errs)
	}
	var records int
	var remaining int
	var status string
	if err := db.QueryRow(`SELECT COUNT(*) FROM redemption_records`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT remaining_times,status FROM member_cards WHERE id=?`, cardID).Scan(&remaining, &status); err != nil {
		t.Fatal(err)
	}
	if records != 1 || remaining != 2 || status != "ACTIVE" {
		t.Fatalf("records=%d remaining=%d status=%s", records, remaining, status)
	}
}

func TestConfirmIsIdempotentForSameRequest(t *testing.T) {
	db := redemptionIntegrationDB(t)
	memberID, staffID, cardID := seedCard(t, db, "COUNT_CARD", "ACTIVE")
	h := &Handler{db: db, now: time.Now}
	token, err := h.createToken(context.Background(), memberID, cardID)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/staff/redemptions/confirm", nil)
	first, err := h.confirm(context.Background(), staffID, token.Token, "redeem-idempotent", req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.confirm(context.Background(), staffID, token.Token, "redeem-idempotent", req)
	if err != nil {
		t.Fatal(err)
	}
	if first.RedemptionNo != second.RedemptionNo {
		t.Fatalf("different results: %s %s", first.RedemptionNo, second.RedemptionNo)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM redemption_records`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected one record, got %d", count)
	}
}

func TestTimePassActivatesAndBlocksSecondUseSameShanghaiDay(t *testing.T) {
	db := redemptionIntegrationDB(t)
	memberID, staffID, cardID := seedCard(t, db, "TIME_PASS", "PENDING_ACTIVATION")
	now := time.Date(2026, 9, 17, 4, 0, 0, 0, time.UTC)
	h := &Handler{db: db, now: func() time.Time { return now }}
	token, err := h.createToken(context.Background(), memberID, cardID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.confirm(context.Background(), staffID, token.Token, "time-pass-first", httptest.NewRequest("POST", "/confirm", nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.CardStatus != "ACTIVE" || result.ActivatedAt == nil || result.ExpiresAt == nil || result.BeforeRemaining != nil || result.AfterRemaining != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := h.createToken(context.Background(), memberID, cardID); !errors.Is(err, errDailyLimit) {
		t.Fatalf("expected daily limit, got %v", err)
	}
}
