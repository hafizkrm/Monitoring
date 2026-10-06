package models

import "time"

type Device struct {
	ID              int        `json:"id" db:"id"`
	Name            string     `json:"name" db:"name"`
	Hostname        string     `json:"hostname" db:"hostname"`
	IPAddress       string     `json:"ip_address" db:"ip_address"`
	DeviceType      string     `json:"device_type" db:"device_type"`
	Vendor          Vendor     `json:"vendor" db:"vendor"`
	SNMPCommunity   string     `json:"snmp_community" db:"snmp_community"`
	SNMPVersion     string     `json:"snmp_version" db:"snmp_version"`
	SNMPUsername    string     `json:"snmp_username" db:"snmp_username"`
	SNMPAuthProto   string     `json:"snmp_auth_proto" db:"snmp_auth_proto"`
	SNMPAuthPass    string     `json:"snmp_auth_pass" db:"snmp_auth_pass"`
	SNMPPrivProto   string     `json:"snmp_priv_proto" db:"snmp_priv_proto"`
	SNMPPrivPass    string     `json:"snmp_priv_pass" db:"snmp_priv_pass"`
	PollingInterval int        `json:"polling_interval" db:"polling_interval"`
	Timeout         int        `json:"timeout" db:"timeout"`
	Enabled            bool       `json:"enabled" db:"enabled"`
	Status             string     `json:"status" db:"status"` // represents overall_status
	ReachabilityStatus string     `json:"reachability_status" db:"reachability_status"`
	SNMPStatus         string     `json:"snmp_status" db:"snmp_status"`
	Location           string     `json:"location" db:"location"`
	ParentIP        string     `json:"parent_ip" db:"parent_ip"`
	LastPolledAt    *time.Time `json:"last_polled_at" db:"last_polled_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}
