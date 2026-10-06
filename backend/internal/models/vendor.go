package models

type Vendor string

const (
	VendorUbiquiti Vendor = "ubiquiti"
	VendorTPLink   Vendor = "tplink"
	VendorMikrotik Vendor = "mikrotik"
	VendorRuijie   Vendor = "ruijie"
	VendorCisco    Vendor = "cisco"
	VendorUnknown  Vendor = "unknown"
)
