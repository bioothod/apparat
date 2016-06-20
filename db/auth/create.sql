USE `apparat.auth`;

CREATE TABLE `users` (
    `username` VARCHAR(128) NULL DEFAULT NULL,
    `password` VARCHAR(64) NULL DEFAULT NULL,
    `realname` VARCHAR(128) NULL DEFAULT NULL,
    `email` VARCHAR(128) NULL DEFAULT NULL,
    `created` DATETIME NULL DEFAULT NULL,
    PRIMARY KEY (`username`),
    UNIQUE (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=UTF8;
