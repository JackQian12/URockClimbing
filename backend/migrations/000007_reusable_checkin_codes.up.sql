CREATE TABLE checkin_codes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code_no VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    token_hash CHAR(64) NOT NULL,
    token_encrypted VARBINARY(512) NOT NULL,
    status ENUM('ACTIVE', 'INACTIVE') NOT NULL DEFAULT 'ACTIVE',
    version INT UNSIGNED NOT NULL DEFAULT 1,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_checkin_code_no (code_no),
    UNIQUE KEY uk_checkin_token_hash (token_hash),
    KEY idx_checkin_status (status, updated_at),
    CONSTRAINT fk_checkin_created_by FOREIGN KEY (created_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE redemption_records
    MODIFY COLUMN token_id BIGINT UNSIGNED NULL,
    ADD COLUMN checkin_code_id BIGINT UNSIGNED NULL AFTER token_id,
    ADD COLUMN redemption_mode ENUM('STAFF_SCAN', 'MEMBER_SELF_SCAN') NOT NULL DEFAULT 'STAFF_SCAN' AFTER checkin_code_id,
    ADD KEY idx_redemption_checkin_time (checkin_code_id, redeemed_at),
    ADD CONSTRAINT fk_redemption_checkin_code FOREIGN KEY (checkin_code_id) REFERENCES checkin_codes (id),
    ADD CONSTRAINT chk_redemption_source CHECK (
        (redemption_mode = 'STAFF_SCAN' AND token_id IS NOT NULL AND checkin_code_id IS NULL) OR
        (redemption_mode = 'MEMBER_SELF_SCAN' AND token_id IS NULL AND checkin_code_id IS NOT NULL)
    );
