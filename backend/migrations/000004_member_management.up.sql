ALTER TABLE users
    ADD COLUMN version INT UNSIGNED NOT NULL DEFAULT 1 AFTER last_login_at;

CREATE TABLE member_profiles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    phone_last4 CHAR(4) NULL,
    phone_authorized_at DATETIME(3) NULL,
    privacy_consent_version VARCHAR(32) NULL,
    privacy_consent_at DATETIME(3) NULL,
    marketing_consent_at DATETIME(3) NULL,
    tags JSON NOT NULL DEFAULT (JSON_ARRAY()),
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_member_profile_user (user_id),
    KEY idx_member_profile_phone_last4 (phone_last4),
    CONSTRAINT fk_member_profile_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO member_profiles (user_id)
SELECT id FROM users WHERE role = 'MEMBER';
