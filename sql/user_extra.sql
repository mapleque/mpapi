DROP TABLE IF EXISTS `user_extra`;
CREATE TABLE `user_extra` (
    `id` INT UNSIGNED NOT NULL PRIMARY KEY AUTO_INCREMENT,
    `user_id` INT UNSIGNED NOT NULL,
    `nickname` VARCHAR(128) NOT NULL DEFAULT '',
    `avatar_url` TEXT,
    `create_at` DATETIME NOT NULL DEFAULT NOW()
) DEFAULT CHARSET=utf8;
ALTER TABLE `user_extra` ADD UNIQUE (`user_id`);
