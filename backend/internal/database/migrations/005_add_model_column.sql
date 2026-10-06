-- Migration 005: Add model column to device_metrics
-- Stores RouterBOARD model (e.g. RB750Gr3, RB951HnD) or Ubiquiti model
ALTER TABLE device_metrics
    ADD COLUMN IF NOT EXISTS model VARCHAR(128) DEFAULT '' AFTER rx_rate;
