ALTER TABLE member_profiles
    DROP KEY idx_member_profile_registered,
    DROP COLUMN registered_at;
