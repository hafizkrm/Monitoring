CREATE TABLE IF NOT EXISTS threshold_rules (
    id INT AUTO_INCREMENT PRIMARY KEY,
    device_id INT NULL,
    metric_name VARCHAR(50) NOT NULL,
    threshold_value FLOAT NOT NULL,
    strike_count INT DEFAULT 1,
    incident_type VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    _unique_device_id INT GENERATED ALWAYS AS (IFNULL(device_id, 0)) VIRTUAL,
    UNIQUE KEY uk_device_metric (_unique_device_id, metric_name),
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Seed default global rules representing M12 behavior
INSERT IGNORE INTO threshold_rules (device_id, metric_name, threshold_value, strike_count, incident_type, is_active) VALUES 
(NULL, 'cpu', 80.0, 1, 'high_cpu', 1),
(NULL, 'memory', 85.0, 1, 'high_ram', 1),
(NULL, 'latency', 80.0, 2, 'high_latency', 1);
