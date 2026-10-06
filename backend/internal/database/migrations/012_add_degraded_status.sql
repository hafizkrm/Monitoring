-- Migration 012: Add degraded status and separate reachability/snmp status
-- Created: 2026-10-05

-- Update ENUM for status in device_metrics
ALTER TABLE device_metrics MODIFY COLUMN status ENUM('up', 'down', 'degraded', 'unknown') DEFAULT 'unknown' COMMENT 'Overall device status';

-- Add specific reachability and snmp status columns
ALTER TABLE device_metrics ADD COLUMN reachability_status ENUM('up', 'down', 'unknown') DEFAULT 'unknown' COMMENT 'ICMP/Ping status' AFTER uptime;
ALTER TABLE device_metrics ADD COLUMN snmp_status ENUM('up', 'down', 'unknown') DEFAULT 'unknown' COMMENT 'SNMP query status' AFTER reachability_status;

-- Update device_status_history ENUM to match
ALTER TABLE device_status_history MODIFY COLUMN status_before ENUM('up', 'down', 'degraded', 'unknown') COMMENT 'Previous status';
ALTER TABLE device_status_history MODIFY COLUMN status_after ENUM('up', 'down', 'degraded', 'unknown') COMMENT 'New status';
