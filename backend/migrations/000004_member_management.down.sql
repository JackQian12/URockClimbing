DROP TABLE IF EXISTS member_profiles;

ALTER TABLE users
    DROP COLUMN version;
