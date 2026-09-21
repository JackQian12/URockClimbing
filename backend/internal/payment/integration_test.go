//go:build integration

package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"urockclimbing.com/backend/internal/order"
	"urockclimbing.com/backend/internal/platform/wechat"
)

func paymentIntegrationDB(t *testing.T) *sql.DB {
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
	for _, table := range []string{"redemption_records", "redemption_tokens", "checkin_codes", "refund_transactions", "member_cards", "payment_transactions", "orders", "admin_sessions", "admin_credentials", "audit_logs", "refresh_tokens", "wechat_identities", "member_profiles", "card_products", "users"} {
		if _, err := db.Exec(`DELETE FROM ` + table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	return db
}

func seedPayable(t *testing.T, db *sql.DB, productType, activationMode string) (string, uint64) {
	t.Helper()
	userResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URPAYTEST','MEMBER','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	if _, err := db.Exec(`INSERT INTO wechat_identities(user_id,appid,openid) VALUES(?,'wx-test-app','open-test')`, userID); err != nil {
		t.Fatal(err)
	}
	var totalTimes any = 10
	if productType == "TIME_PASS" {
		totalTimes = nil
	}
	productResult, err := db.Exec(`INSERT INTO card_products(name,short_description,description,product_type,total_times,validity_days,activation_mode,price_cent,daily_use_limit,transferable,rules,status,sort_order,version) VALUES('测试会员卡','测试','测试',?,?,30,?,8800,1,FALSE,'测试','ON_SALE',0,1)`, productType, totalTimes, activationMode)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ := productResult.LastInsertId()
	snapshot := order.ProductSnapshot{ProductID: strconv.FormatInt(productID, 10), Name: "测试会员卡", ProductType: productType, ValidityDays: 30, ActivationMode: activationMode, PriceCent: 8800, DailyUseLimit: 1}
	if totalTimes != nil {
		value := uint(10)
		snapshot.TotalTimes = &value
	}
	snapshotJSON, _ := json.Marshal(snapshot)
	orderNo := "URORDPAYTEST" + strings.ReplaceAll(productType+activationMode, "_", "")
	orderResult, err := db.Exec(`INSERT INTO orders(order_no,user_id,status,total_amount_cent,product_snapshot,client_request_id,expires_at) VALUES(?,?,'PENDING',8800,?,'pay-test',UTC_TIMESTAMP(3)+INTERVAL 30 MINUTE)`, orderNo, userID, snapshotJSON)
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := orderResult.LastInsertId()
	if _, err := db.Exec(`INSERT INTO payment_transactions(order_id,provider,merchant_order_no,status,amount_cent) VALUES(?,'WECHAT_PAY',?,'CREATED',8800)`, orderID, orderNo); err != nil {
		t.Fatal(err)
	}
	return orderNo, uint64(orderID)
}

func TestSettlePaymentIsTransactionalAndIdempotent(t *testing.T) {
	db := paymentIntegrationDB(t)
	orderNo, orderID := seedPayable(t, db, "COUNT_CARD", "FIRST_USE")
	handler := &Handler{db: db, appID: "wx-test-app", merchantID: "mch-test"}
	result := wechat.PayOrder{AppID: "wx-test-app", MerchantID: "mch-test", OrderNo: orderNo, TransactionID: "wx-transaction-1", TradeState: "SUCCESS", Currency: "CNY", AmountCent: 8800, SuccessTime: time.Now().UTC().Truncate(time.Millisecond)}

	start := make(chan struct{})
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for i := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errs[index] = handler.settlePayment(context.Background(), result)
		}(i)
	}
	close(start)
	wait.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("concurrent settlement errors: %v, %v", errs[0], errs[1])
	}
	var orderStatus, paymentStatus, cardStatus string
	var cardCount int
	var paidAmount uint64
	if err := db.QueryRow(`SELECT status,paid_amount_cent FROM orders WHERE id=?`, orderID).Scan(&orderStatus, &paidAmount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM payment_transactions WHERE order_id=?`, orderID).Scan(&paymentStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*),MIN(status) FROM member_cards WHERE source_order_id=?`, orderID).Scan(&cardCount, &cardStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "PAID" || paidAmount != 8800 || paymentStatus != "SUCCEEDED" || cardCount != 1 || cardStatus != "PENDING_ACTIVATION" {
		t.Fatalf("unexpected settlement: order=%s paid=%d payment=%s cards=%d card_status=%s", orderStatus, paidAmount, paymentStatus, cardCount, cardStatus)
	}
}

func TestSettlePaymentRejectsAmountMismatchWithoutIssuingCard(t *testing.T) {
	db := paymentIntegrationDB(t)
	orderNo, orderID := seedPayable(t, db, "COUNT_CARD", "PURCHASE")
	handler := &Handler{db: db, appID: "wx-test-app", merchantID: "mch-test"}
	err := handler.settlePayment(context.Background(), wechat.PayOrder{AppID: "wx-test-app", MerchantID: "mch-test", OrderNo: orderNo, TransactionID: "wx-transaction-bad", TradeState: "SUCCESS", Currency: "CNY", AmountCent: 1, SuccessTime: time.Now().UTC()})
	if !errors.Is(err, errPaymentMismatch) {
		t.Fatalf("expected payment mismatch, got %v", err)
	}
	var status string
	var cardCount int
	_ = db.QueryRow(`SELECT status FROM orders WHERE id=?`, orderID).Scan(&status)
	_ = db.QueryRow(`SELECT COUNT(*) FROM member_cards WHERE source_order_id=?`, orderID).Scan(&cardCount)
	if status != "PENDING" || cardCount != 0 {
		t.Fatalf("mismatched payment mutated state: order=%s cards=%d", status, cardCount)
	}
}
