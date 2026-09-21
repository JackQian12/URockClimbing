ALTER TABLE redemption_records
    DROP CHECK chk_redemption_source,
    DROP FOREIGN KEY fk_redemption_checkin_code,
    DROP INDEX idx_redemption_checkin_time,
    DROP COLUMN redemption_mode,
    DROP COLUMN checkin_code_id;

DELETE FROM redemption_records WHERE token_id IS NULL;

ALTER TABLE redemption_records
    MODIFY COLUMN token_id BIGINT UNSIGNED NOT NULL;

DROP TABLE IF EXISTS checkin_codes;
