-- Migration 013: Deduplicate interface metrics and add unique constraint
-- Created: 2026-10-05

-- Step 1: Remove existing duplicate samples to allow adding the unique index
DELETE t1 FROM interface_metrics t1
INNER JOIN interface_metrics t2 
WHERE t1.id < t2.id 
  AND t1.device_id = t2.device_id 
  AND t1.interface_name = t2.interface_name 
  AND t1.collected_at = t2.collected_at;

-- Step 2: Add unique index to prevent future duplicates
ALTER TABLE interface_metrics ADD UNIQUE INDEX idx_unique_sample (device_id, interface_name, collected_at);
