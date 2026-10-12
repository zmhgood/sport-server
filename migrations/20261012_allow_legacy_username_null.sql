-- Legacy column is not written by WeChat-only login. Preserve existing values.
ALTER TABLE users
    MODIFY COLUMN username varchar(50)
    CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL;
