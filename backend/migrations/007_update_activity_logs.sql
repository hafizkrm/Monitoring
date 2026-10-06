-- Add user_id column
ALTER TABLE activity_logs ADD COLUMN user_id BIGINT AFTER id;

-- Rename user to username and change length to 100
ALTER TABLE activity_logs CHANGE COLUMN user username VARCHAR(100) NOT NULL DEFAULT 'System';

-- Modify action column to ensure it supports the enums
ALTER TABLE activity_logs MODIFY COLUMN action VARCHAR(20) NOT NULL;
