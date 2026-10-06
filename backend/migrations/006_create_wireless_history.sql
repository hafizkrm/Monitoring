CREATE TABLE wireless_metrics_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    device_id INT NOT NULL,
    signal INT NULL,
    ccq INT NULL,
    noise_floor INT NULL,
    tx_rate DOUBLE NULL,
    rx_rate DOUBLE NULL,
    distance DOUBLE NULL,
    clients INT NULL,
    frequency INT NULL,
    channel INT NULL,
    uptime BIGINT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_device_time (device_id, created_at),
    INDEX idx_created (created_at)
);
