CREATE TABLE IF NOT EXISTS auth_sessions (
    id CHAR(64) PRIMARY KEY,
    user_id INT NOT NULL,
    role_at_issue VARCHAR(20) NOT NULL,
    refresh_token_hash CHAR(64) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    absolute_expires_at DATETIME NULL,
    revoked_at DATETIME NULL,
    INDEX idx_auth_sessions_user_id (user_id),
    INDEX idx_auth_sessions_expiry (expires_at)
);
