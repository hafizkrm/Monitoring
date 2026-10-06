-- Migration 008: Fix Canonical Device ID (BIGINT) and Timezone standards
-- Purpose: 
-- 1. Upgrade devices.id to BIGINT
-- 2. Upgrade all device_id foreign keys to BIGINT
-- 3. Ensure Timezone UTC context

-- Step 1: Drop foreign keys
ALTER TABLE device_metrics DROP FOREIGN KEY device_metrics_ibfk_1;
ALTER TABLE interface_metrics DROP FOREIGN KEY interface_metrics_ibfk_1;
ALTER TABLE device_status_history DROP FOREIGN KEY device_status_history_ibfk_1;
ALTER TABLE polling_logs DROP FOREIGN KEY polling_logs_ibfk_1;
-- activity_logs doesn't have an explicit foreign key in previous schemas, but we'll alter the column anyway

-- Step 2: Alter columns to BIGINT
ALTER TABLE devices MODIFY COLUMN id BIGINT AUTO_INCREMENT;

ALTER TABLE device_metrics MODIFY COLUMN device_id BIGINT NOT NULL;
ALTER TABLE interface_metrics MODIFY COLUMN device_id BIGINT NOT NULL;
ALTER TABLE device_status_history MODIFY COLUMN device_id BIGINT NOT NULL;
ALTER TABLE polling_logs MODIFY COLUMN device_id BIGINT NOT NULL;
ALTER TABLE activity_logs MODIFY COLUMN device_id BIGINT;

-- Step 3: Re-add foreign keys
ALTER TABLE device_metrics ADD CONSTRAINT device_metrics_ibfk_1 FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;
ALTER TABLE interface_metrics ADD CONSTRAINT interface_metrics_ibfk_1 FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;
ALTER TABLE device_status_history ADD CONSTRAINT device_status_history_ibfk_1 FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;
ALTER TABLE polling_logs ADD CONSTRAINT polling_logs_ibfk_1 FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;

-- Step 4: Ensure Global Timezone (Optional for MySQL, but good practice to assert connection timezones)
-- We enforce UTC in the Go backend, but this confirms the database understands it.
SET GLOBAL time_zone = '+00:00';
