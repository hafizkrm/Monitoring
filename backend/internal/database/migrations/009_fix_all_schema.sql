-- ============================================================
-- FIX ALL SCHEMA - Jalankan ini jika database monitoring rusak
-- Tanggal: 2026-06-30
-- ============================================================


-- ============================================================
-- Table: devices
-- ============================================================
CREATE TABLE IF NOT EXISTS devices (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  ip_address VARCHAR(15) NOT NULL,
  snmp_community VARCHAR(255) DEFAULT 'public',
  snmp_version ENUM('1', '2c', '3') DEFAULT '2c',
  polling_interval INT DEFAULT 60,
  timeout INT DEFAULT 5,
  enabled BOOLEAN DEFAULT 1,
  device_type VARCHAR(50),
  location VARCHAR(255),
  parent_ip VARCHAR(15),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  last_polled_at TIMESTAMP NULL,
  INDEX idx_ip_address (ip_address),
  INDEX idx_enabled (enabled),
  INDEX idx_device_type (device_type),
  INDEX idx_name (name),
  INDEX idx_parent_ip (parent_ip)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Table: device_metrics (LENGKAP dengan semua kolom)
-- ============================================================
CREATE TABLE IF NOT EXISTS device_metrics (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  device_id INT NOT NULL,
  cpu_usage DECIMAL(5,2) DEFAULT 0,
  memory_usage DECIMAL(5,2) DEFAULT 0,
  memory_total BIGINT DEFAULT 0,
  memory_used BIGINT DEFAULT 0,
  uptime BIGINT DEFAULT 0,
  status VARCHAR(50) DEFAULT 'down',
  signal_strength DECIMAL(5,2) DEFAULT 0,
  ccq DECIMAL(5,2) DEFAULT 0,
  tx_rate DECIMAL(15,4) DEFAULT 0,
  rx_rate DECIMAL(15,4) DEFAULT 0,
  latency DECIMAL(10,2) DEFAULT 0,
  packet_loss DECIMAL(5,2) DEFAULT 0,
  model VARCHAR(100) DEFAULT '',
  temperature DECIMAL(5,2) DEFAULT 0,
  voltage DECIMAL(6,3) DEFAULT 0,
  collected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_device_collected (device_id, collected_at DESC),
  INDEX idx_collected_at (collected_at),
  INDEX idx_status (status),
  INDEX idx_device_status (device_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Table: interface_metrics
-- ============================================================
CREATE TABLE IF NOT EXISTS interface_metrics (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  device_id INT NOT NULL,
  interface_index INT,
  interface_name VARCHAR(255),
  interface_status VARCHAR(50) DEFAULT 'down',
  in_octets BIGINT DEFAULT 0,
  out_octets BIGINT DEFAULT 0,
  in_errors BIGINT DEFAULT 0,
  out_errors BIGINT DEFAULT 0,
  in_discards BIGINT DEFAULT 0,
  out_discards BIGINT DEFAULT 0,
  interface_speed BIGINT DEFAULT 0,
  rx_mbps DECIMAL(15,4) DEFAULT 0,
  tx_mbps DECIMAL(15,4) DEFAULT 0,
  collected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_device_interface_time (device_id, interface_name, collected_at DESC),
  INDEX idx_interface_name (interface_name),
  INDEX idx_collected_at (collected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Table: device_status_history
-- ============================================================
CREATE TABLE IF NOT EXISTS device_status_history (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  device_id INT NOT NULL,
  status_before VARCHAR(50),
  status_after VARCHAR(50),
  reason VARCHAR(500),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_device_time (device_id, created_at DESC),
  INDEX idx_status_after (status_after),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Table: polling_logs
-- ============================================================
CREATE TABLE IF NOT EXISTS polling_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  device_id INT NOT NULL,
  status ENUM('success', 'timeout', 'error'),
  message VARCHAR(500),
  error_code VARCHAR(50),
  duration_ms INT DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  INDEX idx_device_time (device_id, created_at DESC),
  INDEX idx_status_date (status, created_at DESC),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Table: activity_logs
-- ============================================================
CREATE TABLE IF NOT EXISTS activity_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  device_id INT,
  action VARCHAR(100),
  description TEXT,
  ip_address VARCHAR(15),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_device_time (device_id, created_at DESC),
  INDEX idx_action (action),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- Tambah kolom yang mungkin hilang (IF NOT EXISTS = aman dijalankan ulang)
-- ============================================================
ALTER TABLE device_metrics
  ADD COLUMN IF NOT EXISTS signal_strength DECIMAL(5,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS ccq DECIMAL(5,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tx_rate DECIMAL(15,4) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS rx_rate DECIMAL(15,4) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS latency DECIMAL(10,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS packet_loss DECIMAL(5,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS model VARCHAR(100) DEFAULT '',
  ADD COLUMN IF NOT EXISTS temperature DECIMAL(5,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS voltage DECIMAL(6,3) DEFAULT 0;

ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS snmp_version ENUM('1', '2c', '3') DEFAULT '2c',
  ADD COLUMN IF NOT EXISTS parent_ip VARCHAR(15);

ALTER TABLE interface_metrics
  ADD COLUMN IF NOT EXISTS rx_mbps DECIMAL(15,4) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS tx_mbps DECIMAL(15,4) DEFAULT 0;

SELECT 'Schema fix completed successfully!' AS result;
SHOW TABLES;
