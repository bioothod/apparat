USE `apparat.auth`;

CREATE TABLE `users` (
    `username` VARCHAR(128) NOT NULL,
    `password` VARCHAR(64) NOT NULL,
    `realname` VARCHAR(128) NULL DEFAULT NULL,
    `email` VARCHAR(128) NULL DEFAULT NULL,
    `created` DATETIME NULL DEFAULT NULL,
    PRIMARY KEY (`username`),
    UNIQUE (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=UTF8;
