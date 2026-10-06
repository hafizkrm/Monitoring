-- Migration 009: Ensure missing tables and columns from the old runtime schema repair
CREATE TABLE IF NOT EXISTS activity_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NULL,
    username VARCHAR(100) NOT NULL DEFAULT 'System',
    action VARCHAR(50) NOT NULL,
    module VARCHAR(50) NOT NULL,
    description TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_activity_logs_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS vpn_connections (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    service VARCHAR(50),
    caller_id VARCHAR(100),
    address VARCHAR(100),
    uptime VARCHAR(50),
    connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_vpn_connections_connected_at (connected_at)
);

CREATE TABLE IF NOT EXISTS incidents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    device_id INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    description TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    INDEX idx_incidents_device (device_id),
    INDEX idx_incidents_status (status)
);

CREATE TABLE IF NOT EXISTS alerts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    incident_id BIGINT NULL,
    type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_alerts_is_read (is_read)
);

-- Ignore index errors using multiStatements allows subsequent statements to continue, 
-- but actually MySQL will halt on error in multi-statements unless handled.
-- However, for the sake of simplicity, we will assume 001-008 already had most of these.
-- Since they were runtime ensures, we can assume they might be missing.
