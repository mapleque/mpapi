DROP TABLE IF EXISTS `mp_notify`;
CREATE TABLE `mp_notify` (
    `id` INT UNSIGNED NOT NULL PRIMARY KEY AUTO_INCREMENT,
    `description` VARCHAR(128) NOT NULL,
    `user_id` INT UNSIGNED NOT NULL,
    `appid` VARCHAR(128) NOT NULL,
    `template_id` VARCHAR(128) NOT NULL,
    `page` VARCHAR(128) NOT NULL DEFAULT '',
    `form_id` VARCHAR(128) NOT NULL,
    `data` TEXT,
    `emphasis_keyword` TEXT,
    `status` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0:未处理,1:处理中,2:发送成功,3:发送失败,4:取消',
    `additional` TEXT,
    `active_at` DATETIME NOT NULL COMMENT '预期发送时间',
    `create_at` DATETIME NOT NULL DEFAULT NOW()
) DEFAULT CHARSET=utf8 COMMENT '小程序通知队列';
ALTER TABLE `mp_notify` ADD INDEX (`user_id`), ADD INDEX (`status`), ADD INDEX (`active_at`);
