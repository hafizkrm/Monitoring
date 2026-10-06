-- Migration 003: Add missing columns to devices table
-- Created: 2026-06-01
-- Purpose: Fix missing snmp_version and parent_ip columns

-- Add snmp_version column if it doesn't exist
ALTER TABLE devices 
ADD COLUMN IF NOT EXISTS snmp_version ENUM('1', '2c', '3') DEFAULT '2c' COMMENT 'SNMP version' AFTER snmp_community;

-- Add parent_ip column if it doesn't exist
ALTER TABLE devices 
ADD COLUMN IF NOT EXISTS parent_ip VARCHAR(15) COMMENT 'Parent device IP (for hierarchical monitoring)' AFTER location;

-- Add indexes if needed
ALTER TABLE devices 
ADD INDEX IF NOT EXISTS idx_snmp_version (snmp_version),
ADD INDEX IF NOT EXISTS idx_parent_ip (parent_ip);
