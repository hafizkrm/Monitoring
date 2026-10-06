-- Migration 002: Add Performance Indexes
-- Created: 2026-05-11

ALTER TABLE device_metrics
  ADD INDEX idx_device_collected (device_id, collected_at DESC),
  ADD INDEX idx_collected_at (collected_at),
  ADD INDEX idx_status (status);

ALTER TABLE interface_metrics
  ADD INDEX idx_device_interface_time (device_id, interface_name, collected_at DESC),
  ADD INDEX idx_interface_name (interface_name);

ALTER TABLE device_status_history
  ADD INDEX idx_device_time (device_id, created_at DESC),
  ADD INDEX idx_status_after (status_after);

ALTER TABLE polling_logs
  ADD INDEX idx_device_time (device_id, created_at DESC),
  ADD INDEX idx_status_date (status, created_at DESC);
