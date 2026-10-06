-- Migration 010: Add status matrix columns
ALTER TABLE devices 
ADD COLUMN reachability_status VARCHAR(50) DEFAULT 'unknown' AFTER status,
ADD COLUMN snmp_status VARCHAR(50) DEFAULT 'unknown' AFTER reachability_status;

-- Update existing rows (map current status to matrix)
UPDATE devices SET 
reachability_status = status,
snmp_status = status;
