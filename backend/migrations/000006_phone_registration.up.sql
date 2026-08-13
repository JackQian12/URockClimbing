ALTER TABLE member_profiles
    ADD COLUMN registered_at DATETIME(3) NULL AFTER phone_authorized_at,
    ADD KEY idx_member_profile_registered (registered_at, user_id);

UPDATE member_profiles
SET registered_at = phone_authorized_at
WHERE phone_authorized_at IS NOT NULL;
