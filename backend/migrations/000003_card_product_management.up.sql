ALTER TABLE card_products
    ADD COLUMN short_description VARCHAR(255) NOT NULL DEFAULT '' AFTER name,
    ADD COLUMN list_price_cent BIGINT UNSIGNED NULL AFTER price_cent,
    ADD COLUMN activation_mode ENUM('PURCHASE', 'FIRST_USE') NOT NULL DEFAULT 'PURCHASE' AFTER validity_days,
    ADD COLUMN purchase_limit INT UNSIGNED NULL AFTER activation_mode,
    ADD COLUMN daily_use_limit INT UNSIGNED NOT NULL DEFAULT 1 AFTER purchase_limit,
    ADD COLUMN transferable BOOLEAN NOT NULL DEFAULT FALSE AFTER daily_use_limit,
    ADD COLUMN rules TEXT NOT NULL AFTER transferable,
    ADD COLUMN badge VARCHAR(32) NULL AFTER rules,
    ADD COLUMN theme_color CHAR(7) NOT NULL DEFAULT '#6F4A2E' AFTER badge,
    ADD CONSTRAINT chk_card_product_list_price CHECK (list_price_cent IS NULL OR list_price_cent >= price_cent),
    ADD CONSTRAINT chk_card_product_purchase_limit CHECK (purchase_limit IS NULL OR purchase_limit > 0),
    ADD CONSTRAINT chk_card_product_daily_limit CHECK (daily_use_limit > 0);
