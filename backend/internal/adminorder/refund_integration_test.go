//go:build integration

package adminorder

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"urockclimbing.com/backend/internal/adminauth"
	"urockclimbing.com/backend/internal/platform/wechat"
)

func refundIntegrationDB(t *testing.T) *sql.DB {
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

func seedPaidRefundFixture(t *testing.T, db *sql.DB, cardStatus string) (string, string, uint64) {
	t.Helper()
	adminResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URADMINREFUND','ADMIN','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	adminID, _ := adminResult.LastInsertId()
	memberResult, err := db.Exec(`INSERT INTO users(member_no,role,status) VALUES('URMEMBERREFUND','MEMBER','ACTIVE')`)
	if err != nil {
		t.Fatal(err)
	}
	memberID, _ := memberResult.LastInsertId()
	productResult, err := db.Exec(`INSERT INTO card_products(name,short_description,description,product_type,total_times,validity_days,activation_mode,price_cent,daily_use_limit,transferable,rules,status,sort_order,version) VALUES('退款测试卡','测试','测试','COUNT_CARD',10,30,'FIRST_USE',8800,1,FALSE,'测试','ON_SALE',0,1)`)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ := productResult.LastInsertId()
	orderNo := "URORDREFUNDTEST"
	orderResult, err := db.Exec(`INSERT INTO orders(order_no,user_id,status,total_amount_cent,paid_amount_cent,product_snapshot,client_request_id,expires_at,paid_at) VALUES(?,?,'PAID',8800,8800,JSON_OBJECT('product_id',CAST(? AS CHAR),'name','退款测试卡'),'refund-order-key',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`, orderNo, memberID, productID)
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := orderResult.LastInsertId()
	transactionID := "wx-refund-transaction"
	paymentResult, err := db.Exec(`INSERT INTO payment_transactions(order_id,provider,merchant_order_no,provider_transaction_id,status,amount_cent,succeeded_at) VALUES(?,'WECHAT_PAY',?,?,'SUCCEEDED',8800,UTC_TIMESTAMP(3))`, orderID, orderNo, transactionID)
	if err != nil {
		t.Fatal(err)
	}
	_ = paymentResult
	var activatedAt, expiresAt any
	if cardStatus == "ACTIVE" {
		activatedAt = time.Now().UTC()
		expiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
	}
	if _, err := db.Exec(`INSERT INTO member_cards(card_no,user_id,source_order_id,product_id,product_name,product_type,total_times,remaining_times,status,activated_at,expires_at) VALUES('URCARDREFUND',?,?,?,'退款测试卡','COUNT_CARD',10,10,?,?,?)`, memberID, orderID, productID, cardStatus, activatedAt, expiresAt); err != nil {
		t.Fatal(err)
	}
	return orderNo, transactionID, uint64(adminID)
}

func TestRefundLocksCardAndSuccessFinalizesAtomically(t *testing.T) {
	db := refundIntegrationDB(t)
	orderNo, transactionID, adminID := seedPaidRefundFixture(t, db, "ACTIVE")
	handler := &Handler{db: db, merchantID: "mch-test"}
	request := httptest.NewRequest("POST", "/api/v1/admin/orders/"+orderNo+"/refund", nil)
	record, existing, returnedTransactionID, err := handler.lockForRefund(context.Background(), adminauth.Session{UserID: adminID}, orderNo, "顾客申请", "refund-request-1", request)
	if err != nil || existing || returnedTransactionID != transactionID {
		t.Fatalf("lockForRefund: record=%+v existing=%v transaction=%s err=%v", record, existing, returnedTransactionID, err)
	}
	assertRefundState(t, db, "REFUNDING", "REFUND_LOCKED", "CREATED")
	result := wechat.RefundResult{MerchantID: "mch-test", RefundNo: record.RefundNo, OrderNo: orderNo, TransactionID: transactionID, ProviderRefundID: "wx-refund-1", Status: "SUCCESS", Currency: "CNY", TotalCent: 8800, RefundCent: 8800, SuccessTime: time.Now().UTC()}
	if err := handler.applyRefundResult(context.Background(), result, "refund-notify-test"); err != nil {
		t.Fatal(err)
	}
	assertRefundState(t, db, "REFUNDED", "REFUNDED", "SUCCEEDED")
	if err := handler.applyRefundResult(context.Background(), result, "refund-notify-duplicate"); err != nil {
		t.Fatalf("duplicate success must be idempotent: %v", err)
	}
}

func TestExplicitClosedRefundRestoresPendingActivation(t *testing.T) {
	db := refundIntegrationDB(t)
	orderNo, transactionID, adminID := seedPaidRefundFixture(t, db, "PENDING_ACTIVATION")
	handler := &Handler{db: db, merchantID: "mch-test"}
	record, _, _, err := handler.lockForRefund(context.Background(), adminauth.Session{UserID: adminID}, orderNo, "原路退款", "refund-request-2", httptest.NewRequest("POST", "/refund", nil))
	if err != nil {
		t.Fatal(err)
	}
	result := wechat.RefundResult{MerchantID: "mch-test", RefundNo: record.RefundNo, OrderNo: orderNo, TransactionID: transactionID, ProviderRefundID: "wx-refund-closed", Status: "CLOSED", Currency: "CNY", TotalCent: 8800, RefundCent: 8800}
	if err := handler.applyRefundResult(context.Background(), result, "refund-query-test"); err != nil {
		t.Fatal(err)
	}
	assertRefundState(t, db, "PAID", "PENDING_ACTIVATION", "FAILED")
}

func assertRefundState(t *testing.T, db *sql.DB, orderStatus, cardStatus, refundStatus string) {
	t.Helper()
	var actualOrder, actualCard, actualRefund string
	if err := db.QueryRow(`SELECT status FROM orders LIMIT 1`).Scan(&actualOrder); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM member_cards LIMIT 1`).Scan(&actualCard); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM refund_transactions LIMIT 1`).Scan(&actualRefund); err != nil {
		t.Fatal(err)
	}
	if actualOrder != orderStatus || actualCard != cardStatus || actualRefund != refundStatus {
		t.Fatalf("unexpected state: order=%s card=%s refund=%s", actualOrder, actualCard, actualRefund)
	}
}
