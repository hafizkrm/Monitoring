-- Create missing tables and seed users for nms_db
-- Run: mysql -u root -h 127.0.0.1 -P 3306 nms_db < seed.sql

USE nms_db;

-- ============================================================
-- Table: users
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'viewer',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- ============================================================
-- Table: settings
-- ============================================================
CREATE TABLE IF NOT EXISTS settings (
    setting_key VARCHAR(100) PRIMARY KEY,
    setting_value JSON NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- ============================================================
-- Table: wireless_history (migration 006)
-- ============================================================
CREATE TABLE IF NOT EXISTS wireless_history (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    device_id VARCHAR(255) NOT NULL,
    interface_name VARCHAR(255),
    signal_strength DECIMAL(5,2),
    ccq DECIMAL(5,2),
    tx_rate DECIMAL(15,4),
    rx_rate DECIMAL(15,4),
    collected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_device_wireless_time (device_id, collected_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- Seed: Admin & Users
-- Password admin123 → $2a$10$cjxxXPOvishnHcQ.tm837ugUDfBaRjlsCf272V8Y/IneJ4cZWslSe
-- Password hafiz    → $2a$10$gmUm5kUnrVXm.9v5vBhZwe1y/V1KqE3.5j65wck65OaaHBt0RVyfy
-- ============================================================
INSERT IGNORE INTO users (name, username, password_hash, role) VALUES
    ('Administrator', 'admin', '$2a$10$cjxxXPOvishnHcQ.tm837ugUDfBaRjlsCf272V8Y/IneJ4cZWslSe', 'admin'),
    ('Hafiz', 'hafiz', '$2a$10$gmUm5kUnrVXm.9v5vBhZwe1y/V1KqE3.5j65wck65OaaHBt0RVyfy', 'admin');

SELECT 'DONE! Tables created and users seeded.' AS result;
SHOW TABLES;
SELECT id, username, role, created_at FROM users;
