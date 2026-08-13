ALTER TABLE card_products
    DROP CHECK chk_card_product_times,
    ADD COLUMN product_type ENUM('COUNT_CARD', 'TIME_PASS') NOT NULL DEFAULT 'COUNT_CARD' AFTER description,
    MODIFY COLUMN total_times INT UNSIGNED NULL,
    ADD CONSTRAINT chk_card_product_times CHECK (
        (product_type = 'COUNT_CARD' AND total_times IS NOT NULL AND total_times > 0) OR
        (product_type = 'TIME_PASS' AND total_times IS NULL)
    ),
    ADD CONSTRAINT chk_time_pass_rules CHECK (
        product_type <> 'TIME_PASS' OR
        (validity_days IN (7, 30, 90, 365) AND activation_mode = 'FIRST_USE' AND daily_use_limit = 1 AND transferable = FALSE)
    );

ALTER TABLE member_cards
    DROP CHECK chk_member_card_total_times,
    DROP CHECK chk_member_card_remaining,
    ADD COLUMN product_type ENUM('COUNT_CARD', 'TIME_PASS') NOT NULL DEFAULT 'COUNT_CARD' AFTER product_name,
    MODIFY COLUMN total_times INT UNSIGNED NULL,
    MODIFY COLUMN remaining_times INT UNSIGNED NULL,
    MODIFY COLUMN status ENUM('PENDING_ACTIVATION', 'ACTIVE', 'USED_UP', 'EXPIRED', 'REFUND_LOCKED', 'REFUNDED') NOT NULL DEFAULT 'ACTIVE',
    MODIFY COLUMN activated_at DATETIME(3) NULL,
    MODIFY COLUMN expires_at DATETIME(3) NULL,
    ADD CONSTRAINT chk_member_card_balance CHECK (
        (product_type = 'COUNT_CARD' AND total_times IS NOT NULL AND total_times > 0 AND remaining_times IS NOT NULL AND remaining_times <= total_times) OR
        (product_type = 'TIME_PASS' AND total_times IS NULL AND remaining_times IS NULL)
    ),
    ADD CONSTRAINT chk_member_card_activation CHECK (
        (status = 'PENDING_ACTIVATION' AND activated_at IS NULL AND expires_at IS NULL) OR
        (status = 'ACTIVE' AND activated_at IS NOT NULL AND expires_at IS NOT NULL) OR
        (status IN ('USED_UP', 'EXPIRED', 'REFUND_LOCKED', 'REFUNDED'))
    );

ALTER TABLE redemption_records
    DROP CHECK chk_redemption_balance,
    MODIFY COLUMN before_remaining INT UNSIGNED NULL,
    MODIFY COLUMN after_remaining INT UNSIGNED NULL,
    ADD CONSTRAINT chk_redemption_balance CHECK (
        (before_remaining IS NULL AND after_remaining IS NULL) OR
        (before_remaining = after_remaining + times)
    );
