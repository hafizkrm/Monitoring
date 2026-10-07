-- Migration 001: Initialize Database Schema
-- Created: 2026-05-11

-- ============================================================
-- Table: devices
-- Purpose: Store network devices to monitor
-- ============================================================
CREATE TABLE IF NOT EXISTS devices (
  id INT PRIMARY KEY AUTO_INCREMENT,
  
  -- Identifikasi
  name VARCHAR(255) NOT NULL UNIQUE COMMENT 'Device name',
  ip_address VARCHAR(15) NOT NULL COMMENT 'IP address',
  snmp_community VARCHAR(255) DEFAULT 'public' COMMENT 'SNMP community string',
  snmp_version ENUM('1', '2c', '3') DEFAULT '2c' COMMENT 'SNMP version',
  
  -- Konfigurasi Polling
  polling_interval INT DEFAULT 60 COMMENT 'Polling interval in seconds',
  timeout INT DEFAULT 5 COMMENT 'Timeout in seconds',
  enabled BOOLEAN DEFAULT 1 COMMENT 'Enable/disable monitoring',
  
  -- Metadata
  device_type VARCHAR(50) COMMENT 'Type: router, switch, ap, server',
  location VARCHAR(255) COMMENT 'Physical location',
  
  -- Timestamps
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Record creation time',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last update time',
  last_polled_at TIMESTAMP NULL COMMENT 'Last polling time',
  
  -- Indexes untuk query optimization
  INDEX idx_ip_address (ip_address),
  INDEX idx_enabled (enabled),
  INDEX idx_device_type (device_type),
  INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Network devices to monitor';

-- ============================================================
-- Table: device_metrics
-- Purpose: Store CPU, RAM, and system metrics
-- ============================================================
CREATE TABLE IF NOT EXISTS device_metrics (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  
  -- Foreign Key
  device_id INT NOT NULL COMMENT 'Reference to device',
  
  -- System Metrics
  cpu_usage DECIMAL(5,2) COMMENT 'CPU usage percentage (0-100)',
  memory_usage DECIMAL(5,2) COMMENT 'Memory usage percentage (0-100)',
  memory_total BIGINT COMMENT 'Total memory in bytes',
  memory_used BIGINT COMMENT 'Used memory in bytes',
  uptime BIGINT COMMENT 'Device uptime in seconds',
  status VARCHAR(50) DEFAULT 'down' COMMENT 'Device status',
  
  -- Timestamps
  collected_at TIMESTAMP NOT NULL COMMENT 'Time data was collected',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Record creation time',
  
  -- Foreign Key Constraint
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  
  -- Indexes untuk time-series queries
  INDEX idx_device_collected (device_id, collected_at DESC),
  INDEX idx_collected_at (collected_at),
  INDEX idx_status (status),
  INDEX idx_device_status (device_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Device metrics (CPU, RAM, etc)';

-- ============================================================
-- Table: interface_metrics
-- Purpose: Store network interface metrics (bandwidth, errors)
-- ============================================================
CREATE TABLE IF NOT EXISTS interface_metrics (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  
  -- Foreign Key
  device_id INT NOT NULL COMMENT 'Reference to device',
  
  -- Interface Info
  interface_index INT COMMENT 'SNMP interface index',
  interface_name VARCHAR(255) COMMENT 'Interface name (e.g., eth0, ge-0/0/0)',
  interface_status VARCHAR(50) DEFAULT 'down' COMMENT 'Interface status',
  
  -- Traffic Data
  in_octets BIGINT COMMENT 'Bytes received',
  out_octets BIGINT COMMENT 'Bytes transmitted',
  in_errors BIGINT DEFAULT 0 COMMENT 'Input errors',
  out_errors BIGINT DEFAULT 0 COMMENT 'Output errors',
  in_discards BIGINT DEFAULT 0 COMMENT 'Inbound discards',
  out_discards BIGINT DEFAULT 0 COMMENT 'Outbound discards',
  
  -- Speed Info
  interface_speed BIGINT COMMENT 'Interface speed in bits per second',
  
  -- Timestamps
  collected_at TIMESTAMP NOT NULL COMMENT 'Time data was collected',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Record creation time',
  
  -- Foreign Key Constraint
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  
  -- Indexes
  INDEX idx_device_interface_time (device_id, interface_name, collected_at DESC),
  INDEX idx_interface_name (interface_name),
  INDEX idx_collected_at (collected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Network interface metrics (bandwidth, errors)';

-- ============================================================
-- Table: device_status_history
-- Purpose: Track device status changes for alerting
-- ============================================================
CREATE TABLE IF NOT EXISTS device_status_history (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  
  -- Foreign Key
  device_id INT NOT NULL COMMENT 'Reference to device',
  
  -- Status Change
  status_before VARCHAR(50) COMMENT 'Previous status',
  status_after VARCHAR(50) COMMENT 'New status',
  
  -- Details
  reason VARCHAR(500) COMMENT 'Reason for status change',
  
  -- Timestamp
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When change occurred',
  
  -- Foreign Key Constraint
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  
  -- Indexes
  INDEX idx_device_time (device_id, created_at DESC),
  INDEX idx_status_after (status_after),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Device status change history';

-- ============================================================
-- Table: polling_logs
-- Purpose: Log polling attempts for debugging and monitoring
-- ============================================================
CREATE TABLE IF NOT EXISTS polling_logs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  
  -- Foreign Key
  device_id INT NOT NULL COMMENT 'Reference to device',
  
  -- Polling Result
  status ENUM('success', 'timeout', 'error') COMMENT 'Polling result',
  message VARCHAR(500) COMMENT 'Details message',
  error_code VARCHAR(50) COMMENT 'Error code if applicable',
  
  -- Performance
  duration_ms INT COMMENT 'Duration in milliseconds',
  
  -- Timestamp
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'When poll occurred',
  
  -- Foreign Key Constraint
  FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
  
  -- Indexes
  INDEX idx_device_time (device_id, created_at DESC),
  INDEX idx_status_date (status, created_at DESC),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Polling operation logs';

-- ============================================================
-- Event: Auto-cleanup old metrics (optional but recommended)
-- Purpose: Maintain data retention policy (keep 30 days)
-- ============================================================
CREATE EVENT IF NOT EXISTS cleanup_old_metrics
ON SCHEDULE EVERY 1 DAY
STARTS CURRENT_TIMESTAMP
DO
  BEGIN
    DELETE FROM device_metrics 
    WHERE collected_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
    
    DELETE FROM interface_metrics 
    WHERE collected_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
    
    DELETE FROM polling_logs 
    WHERE created_at < DATE_SUB(NOW(), INTERVAL 7 DAY);
    
    DELETE FROM device_status_history 
    WHERE created_at < DATE_SUB(NOW(), INTERVAL 90 DAY);
  END;
