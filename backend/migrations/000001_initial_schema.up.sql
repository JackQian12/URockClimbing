CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    member_no VARCHAR(32) NOT NULL,
    nickname VARCHAR(64) NULL,
    avatar_url VARCHAR(512) NULL,
    phone_encrypted VARBINARY(512) NULL,
    role ENUM('MEMBER', 'STAFF', 'ADMIN') NOT NULL DEFAULT 'MEMBER',
    status ENUM('ACTIVE', 'DISABLED') NOT NULL DEFAULT 'ACTIVE',
    last_login_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_member_no (member_no),
    KEY idx_users_phone_encrypted (phone_encrypted(64))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wechat_identities (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    appid VARCHAR(64) NOT NULL,
    openid VARCHAR(128) NOT NULL,
    unionid VARCHAR(128) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wechat_appid_openid (appid, openid),
    KEY idx_wechat_unionid (unionid),
    CONSTRAINT fk_wechat_identity_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE refresh_tokens (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    token_hash CHAR(64) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    replaced_by_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_refresh_token_hash (token_hash),
    KEY idx_refresh_token_user (user_id, expires_at),
    CONSTRAINT fk_refresh_token_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_refresh_token_replacement FOREIGN KEY (replaced_by_id) REFERENCES refresh_tokens (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE card_products (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    total_times INT UNSIGNED NOT NULL,
    validity_days INT UNSIGNED NOT NULL,
    price_cent BIGINT UNSIGNED NOT NULL,
    status ENUM('DRAFT', 'ON_SALE', 'OFF_SALE') NOT NULL DEFAULT 'DRAFT',
    sort_order INT NOT NULL DEFAULT 0,
    version INT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_card_products_sale_order (status, sort_order, id),
    CONSTRAINT chk_card_product_times CHECK (total_times > 0),
    CONSTRAINT chk_card_product_validity CHECK (validity_days > 0),
    CONSTRAINT chk_card_product_price CHECK (price_cent > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no VARCHAR(64) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    biz_type VARCHAR(32) NOT NULL DEFAULT 'CARD_PURCHASE',
    status ENUM('PENDING', 'PAID', 'CLOSED', 'REFUNDING', 'REFUNDED') NOT NULL DEFAULT 'PENDING',
    total_amount_cent BIGINT UNSIGNED NOT NULL,
    paid_amount_cent BIGINT UNSIGNED NOT NULL DEFAULT 0,
    product_snapshot JSON NOT NULL,
    client_request_id VARCHAR(64) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    paid_at DATETIME(3) NULL,
    closed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_orders_order_no (order_no),
    UNIQUE KEY uk_orders_user_request (user_id, client_request_id),
    KEY idx_orders_user_created (user_id, created_at),
    KEY idx_orders_status_expires (status, expires_at),
    CONSTRAINT fk_order_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT chk_order_total_amount CHECK (total_amount_cent > 0),
    CONSTRAINT chk_order_paid_amount CHECK (paid_amount_cent <= total_amount_cent)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE payment_transactions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id BIGINT UNSIGNED NOT NULL,
    provider ENUM('WECHAT_PAY') NOT NULL,
    merchant_order_no VARCHAR(64) NOT NULL,
    provider_transaction_id VARCHAR(128) NULL,
    status ENUM('CREATED', 'SUCCEEDED', 'FAILED', 'CLOSED') NOT NULL DEFAULT 'CREATED',
    amount_cent BIGINT UNSIGNED NOT NULL,
    callback_payload JSON NULL,
    succeeded_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_merchant_order_no (merchant_order_no),
    UNIQUE KEY uk_payment_provider_transaction (provider_transaction_id),
    KEY idx_payment_order (order_id),
    CONSTRAINT fk_payment_order FOREIGN KEY (order_id) REFERENCES orders (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE member_cards (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    card_no VARCHAR(64) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    source_order_id BIGINT UNSIGNED NOT NULL,
    product_id BIGINT UNSIGNED NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    total_times INT UNSIGNED NOT NULL,
    remaining_times INT UNSIGNED NOT NULL,
    status ENUM('ACTIVE', 'USED_UP', 'EXPIRED', 'REFUND_LOCKED', 'REFUNDED') NOT NULL DEFAULT 'ACTIVE',
    activated_at DATETIME(3) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    version INT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_member_card_no (card_no),
    UNIQUE KEY uk_member_card_source_order (source_order_id),
    KEY idx_member_cards_user_status (user_id, status, expires_at),
    CONSTRAINT fk_member_card_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_member_card_order FOREIGN KEY (source_order_id) REFERENCES orders (id),
    CONSTRAINT fk_member_card_product FOREIGN KEY (product_id) REFERENCES card_products (id),
    CONSTRAINT chk_member_card_total_times CHECK (total_times > 0),
    CONSTRAINT chk_member_card_remaining CHECK (remaining_times <= total_times)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE redemption_tokens (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    token_hash CHAR(64) NOT NULL,
    member_card_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    status ENUM('ACTIVE', 'USED', 'EXPIRED', 'REVOKED') NOT NULL DEFAULT 'ACTIVE',
    expires_at DATETIME(3) NOT NULL,
    used_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_redemption_token_hash (token_hash),
    KEY idx_redemption_token_card_status (member_card_id, status, expires_at),
    CONSTRAINT fk_redemption_token_card FOREIGN KEY (member_card_id) REFERENCES member_cards (id),
    CONSTRAINT fk_redemption_token_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE redemption_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    redemption_no VARCHAR(64) NOT NULL,
    member_card_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    operator_user_id BIGINT UNSIGNED NOT NULL,
    token_id BIGINT UNSIGNED NOT NULL,
    times INT UNSIGNED NOT NULL DEFAULT 1,
    before_remaining INT UNSIGNED NOT NULL,
    after_remaining INT UNSIGNED NOT NULL,
    redeemed_at DATETIME(3) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_redemption_no (redemption_no),
    UNIQUE KEY uk_redemption_token (token_id),
    UNIQUE KEY uk_redemption_operator_request (operator_user_id, request_id),
    KEY idx_redemption_user_time (user_id, redeemed_at),
    KEY idx_redemption_operator_time (operator_user_id, redeemed_at),
    CONSTRAINT fk_redemption_card FOREIGN KEY (member_card_id) REFERENCES member_cards (id),
    CONSTRAINT fk_redemption_user FOREIGN KEY (user_id) REFERENCES users (id),
    CONSTRAINT fk_redemption_operator FOREIGN KEY (operator_user_id) REFERENCES users (id),
    CONSTRAINT fk_redemption_token FOREIGN KEY (token_id) REFERENCES redemption_tokens (id),
    CONSTRAINT chk_redemption_once CHECK (times = 1),
    CONSTRAINT chk_redemption_balance CHECK (before_remaining = after_remaining + times)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE refund_transactions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    refund_no VARCHAR(64) NOT NULL,
    order_id BIGINT UNSIGNED NOT NULL,
    payment_transaction_id BIGINT UNSIGNED NOT NULL,
    provider_refund_id VARCHAR(128) NULL,
    status ENUM('CREATED', 'PROCESSING', 'SUCCEEDED', 'FAILED', 'ABNORMAL') NOT NULL DEFAULT 'CREATED',
    amount_cent BIGINT UNSIGNED NOT NULL,
    reason VARCHAR(255) NOT NULL,
    operator_user_id BIGINT UNSIGNED NOT NULL,
    client_request_id VARCHAR(64) NOT NULL,
    callback_payload JSON NULL,
    succeeded_at DATETIME(3) NULL,
    failed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_refund_no (refund_no),
    UNIQUE KEY uk_refund_order (order_id),
    UNIQUE KEY uk_refund_provider_id (provider_refund_id),
    UNIQUE KEY uk_refund_operator_request (operator_user_id, client_request_id),
    CONSTRAINT fk_refund_order FOREIGN KEY (order_id) REFERENCES orders (id),
    CONSTRAINT fk_refund_payment FOREIGN KEY (payment_transaction_id) REFERENCES payment_transactions (id),
    CONSTRAINT fk_refund_operator FOREIGN KEY (operator_user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    operator_user_id BIGINT UNSIGNED NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    before_snapshot JSON NULL,
    after_snapshot JSON NULL,
    request_ip VARCHAR(64) NULL,
    request_id VARCHAR(128) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_audit_resource (resource_type, resource_id, created_at),
    KEY idx_audit_operator (operator_user_id, created_at),
    CONSTRAINT fk_audit_operator FOREIGN KEY (operator_user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

