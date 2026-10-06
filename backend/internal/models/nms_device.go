package models

import "time"

type NMSDevice struct {
	ID           string    `json:"id" db:"id"`
	Hostname     string    `json:"hostname" db:"hostname"`
	Vendor       string    `json:"vendor" db:"vendor"` // mikrotik | cisco | ubiquiti | tplink
	Type         string    `json:"type" db:"type"`     // router | switch | ap | radio
	Model        string    `json:"model" db:"model"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	SerialNumber string    `json:"serial_number" db:"serial_number"`
	Firmware     string    `json:"firmware" db:"firmware"`
	SiteID       string    `json:"site_id" db:"site_id"`
	Status       string    `json:"status" db:"status"` // online | offline | warning
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
