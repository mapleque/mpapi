DROP TABLE IF EXISTS `user_mobile`;
CREATE TABLE `user_mobile` (
    `id` INT UNSIGNED NOT NULL PRIMARY KEY AUTO_INCREMENT,
    `user_id` INT UNSIGNED NOT NULL,
    `phone_number` VARCHAR(128) DEFAULT '',
    `pure_phone_number` VARCHAR(128) DEFAULT '',
    `country_code` VARCHAR(128) DEFAULT '',
    `create_at` DATETIME NOT NULL DEFAULT NOW()
) DEFAULT CHARSET=utf8;
ALTER TABLE `user_mobile` ADD UNIQUE (`user_id`);

