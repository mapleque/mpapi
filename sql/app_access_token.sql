DROP TABLE IF EXISTS `app_access_token`;
CREATE TABLE `app_access_token` (
    `id` INT UNSIGNED NOT NULL PRIMARY KEY AUTO_INCREMENT,
    `appid` VARCHAR(128) NOT NULL,
    `access_token` TEXT NOT NULL,
    `expires_in` INT UNSIGNED NOT NULL COMMENT '超时时间',
    `refresh` BOOLEAN DEFAULT 0 COMMENT'是否正在刷新',
    `need_refresh` BOOLEAN DEFAULT 0 COMMENT'是否需要刷新',
    `update_at` DATETIME NOT NULL COMMENT '更新时间',
    `create_at` DATETIME NOT NULL DEFAULT NOW()
) DEFAULT CHARSET=utf8;
ALTER TABLE `app_access_token` ADD UNIQUE(`appid`);
