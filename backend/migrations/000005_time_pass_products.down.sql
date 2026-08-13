ALTER TABLE redemption_records
    DROP CHECK chk_redemption_balance;

UPDATE redemption_records
SET before_remaining = 1, after_remaining = 0
WHERE before_remaining IS NULL OR after_remaining IS NULL;

ALTER TABLE redemption_records
    MODIFY COLUMN before_remaining INT UNSIGNED NOT NULL,
    MODIFY COLUMN after_remaining INT UNSIGNED NOT NULL,
    ADD CONSTRAINT chk_redemption_balance CHECK (before_remaining = after_remaining + times);

ALTER TABLE member_cards
    DROP CHECK chk_member_card_balance,
    DROP CHECK chk_member_card_activation;

UPDATE member_cards
SET total_times = 1,
    remaining_times = CASE WHEN status IN ('EXPIRED', 'REFUNDED') THEN 0 ELSE 1 END,
    activated_at = COALESCE(activated_at, created_at),
    expires_at = COALESCE(expires_at, DATE_ADD(created_at, INTERVAL 1 DAY)),
    status = CASE WHEN status = 'PENDING_ACTIVATION' THEN 'ACTIVE' ELSE status END
WHERE product_type = 'TIME_PASS';

UPDATE member_cards mc
JOIN card_products cp ON cp.id = mc.product_id
SET mc.activated_at = COALESCE(mc.activated_at, mc.created_at),
    mc.expires_at = COALESCE(mc.expires_at, DATE_ADD(mc.created_at, INTERVAL cp.validity_days DAY)),
    mc.status = 'ACTIVE'
WHERE mc.status = 'PENDING_ACTIVATION';

ALTER TABLE member_cards
    MODIFY COLUMN total_times INT UNSIGNED NOT NULL,
    MODIFY COLUMN remaining_times INT UNSIGNED NOT NULL,
    MODIFY COLUMN status ENUM('ACTIVE', 'USED_UP', 'EXPIRED', 'REFUND_LOCKED', 'REFUNDED') NOT NULL DEFAULT 'ACTIVE',
    MODIFY COLUMN activated_at DATETIME(3) NOT NULL,
    MODIFY COLUMN expires_at DATETIME(3) NOT NULL,
    DROP COLUMN product_type,
    ADD CONSTRAINT chk_member_card_total_times CHECK (total_times > 0),
    ADD CONSTRAINT chk_member_card_remaining CHECK (remaining_times <= total_times);

ALTER TABLE card_products
    DROP CHECK chk_card_product_times,
    DROP CHECK chk_time_pass_rules;

UPDATE card_products SET total_times = 1 WHERE product_type = 'TIME_PASS';

ALTER TABLE card_products
    MODIFY COLUMN total_times INT UNSIGNED NOT NULL,
    DROP COLUMN product_type,
    ADD CONSTRAINT chk_card_product_times CHECK (total_times > 0);
