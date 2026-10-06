package snmp

// OID (Object Identifier) constants for SNMP queries
// Organized by category for easy reference

const (
	// === System Information ===
	OIDSysUptime   = "1.3.6.1.2.1.1.3.0" // System Uptime (TimeTicks)
	OIDSysName     = "1.3.6.1.2.1.1.5.0" // System Name / Hostname
	OIDSysDescr    = "1.3.6.1.2.1.1.1.0" // System Description
	OIDSysContact  = "1.3.6.1.2.1.1.4.0" // System Contact
	OIDSysLocation = "1.3.6.1.2.1.1.6.0" // System Location
	OIDSysObjectID = "1.3.6.1.2.1.1.2.0" // System Object ID
	OIDSysServices = "1.3.6.1.2.1.1.7.0" // System Services

	// === CPU & Processor Information (HOST-RESOURCES-MIB) ===
	// Note: CPU metrics are difficult to standardize across vendors
	// Different vendors use different OIDs
	OIDCpuLoad1Min  = "1.3.6.1.4.1.2021.10.1.3.1" // CPU Load 1 min (UCD-SNMP)
	OIDCpuLoad5Min  = "1.3.6.1.4.1.2021.10.1.3.2" // CPU Load 5 min (UCD-SNMP)
	OIDCpuLoad15Min = "1.3.6.1.4.1.2021.10.1.3.3" // CPU Load 15 min (UCD-SNMP)

	// === Memory Information (HOST-RESOURCES-MIB) ===
	OIDMemorySize = "1.3.6.1.2.1.25.2.2.0"     // Total Memory
	OIDMemoryUsed = "1.3.6.1.2.1.25.2.3.1.5.1" // Used Memory
	OIDMemoryFree = "1.3.6.1.2.1.25.2.3.1.5.2" // Free Memory

	// Alternative memory OIDs (for some devices)
	OIDStorageIndex           = "1.3.6.1.2.1.25.2.3.1.1" // Storage Index
	OIDStorageType            = "1.3.6.1.2.1.25.2.3.1.2" // Storage Type
	OIDStorageDescr           = "1.3.6.1.2.1.25.2.3.1.3" // Storage Description
	OIDStorageAllocationUnits = "1.3.6.1.2.1.25.2.3.1.4" // Allocation Units
	OIDStorageSize            = "1.3.6.1.2.1.25.2.3.1.5" // Storage Size
	OIDStorageUsed            = "1.3.6.1.2.1.25.2.3.1.6" // Storage Used
	OIDStorageFree            = "1.3.6.1.2.1.25.2.3.1.7" // Storage Free

	// === Interface Information (IF-MIB) ===
	OIDIfNumber          = "1.3.6.1.2.1.2.1.0"    // Number of interfaces
	OIDIfIndex           = "1.3.6.1.2.1.2.2.1.1"  // Interface Index
	OIDIfDescr           = "1.3.6.1.2.1.2.2.1.2"  // Interface Description
	OIDIfType            = "1.3.6.1.2.1.2.2.1.3"  // Interface Type
	OIDIfMTU             = "1.3.6.1.2.1.2.2.1.4"  // Interface MTU
	OIDIfSpeed           = "1.3.6.1.2.1.2.2.1.5"  // Interface Speed (bits/sec)
	OIDIfPhysAddress     = "1.3.6.1.2.1.2.2.1.6"  // Physical Address (MAC)
	OIDIfAdminStatus     = "1.3.6.1.2.1.2.2.1.7"  // Admin Status (up/down/testing)
	OIDIfOperStatus      = "1.3.6.1.2.1.2.2.1.8"  // Operational Status (up/down/testing)
	OIDIfLastChange      = "1.3.6.1.2.1.2.2.1.9"  // Last Change
	OIDIfInOctets        = "1.3.6.1.2.1.2.2.1.10" // Input Octets (32-bit)
	OIDIfInUcastPkts     = "1.3.6.1.2.1.2.2.1.11" // Input Unicast Packets
	OIDIfInNUcastPkts    = "1.3.6.1.2.1.2.2.1.12" // Input Non-unicast Packets
	OIDIfInDiscards      = "1.3.6.1.2.1.2.2.1.13" // Input Discards
	OIDIfInErrors        = "1.3.6.1.2.1.2.2.1.14" // Input Errors
	OIDIfInUnknownProtos = "1.3.6.1.2.1.2.2.1.15" // Input Unknown Protocols
	OIDIfOutOctets       = "1.3.6.1.2.1.2.2.1.16" // Output Octets (32-bit)
	OIDIfOutUcastPkts    = "1.3.6.1.2.1.2.2.1.17" // Output Unicast Packets
	OIDIfOutNUcastPkts   = "1.3.6.1.2.1.2.2.1.18" // Output Non-unicast Packets
	OIDIfOutDiscards     = "1.3.6.1.2.1.2.2.1.19" // Output Discards
	OIDIfOutErrors       = "1.3.6.1.2.1.2.2.1.20" // Output Errors
	OIDIfOutQLen         = "1.3.6.1.2.1.2.2.1.21" // Output Queue Length

	// High Capacity 64-bit Interface Counters (IF-MIB)
	OIDIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"  // High Capacity Inbound Octets (64-bit)
	OIDIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10" // High Capacity Outbound Octets (64-bit)
	OIDIfHighSpeed   = "1.3.6.1.2.1.31.1.1.1.15" // High Capacity Interface Speed (Mbps)
	OIDIfName        = "1.3.6.1.2.1.31.1.1.1.1"  // Interface Name
	OIDIfAlias       = "1.3.6.1.2.1.31.1.1.1.18" // Interface Alias (Description)

	// === Cisco Specific OIDs (For Routers/Switches) ===
	OIDCiscoMemoryPoolUsed = "1.3.6.1.4.1.9.9.48.1.1.1.5.1"    // Cisco Memory Used
	OIDCiscoMemoryPoolFree = "1.3.6.1.4.1.9.9.48.1.1.1.6.1"    // Cisco Memory Free
	OIDCiscoCPUUsage5Min   = "1.3.6.1.4.1.9.9.109.1.1.1.1.8.1" // Cisco CPU 5-min

	// === SNMP System Group ===
	OIDSysUpTimeInstance = "1.3.6.1.2.1.1.3.0" // Uptime

	// === IP Statistics (IP-MIB) ===
	OIDIpInReceives  = "1.3.6.1.2.1.4.3.0"  // IP Packets Received
	OIDIpInDelivers  = "1.3.6.1.2.1.4.9.0"  // IP Packets Delivered
	OIDIpOutRequests = "1.3.6.1.2.1.4.10.0" // IP Packets Sent
	OIDIpInDiscards  = "1.3.6.1.2.1.4.8.0"  // IP Packets Discarded (input)
	OIDIpOutDiscards = "1.3.6.1.2.1.4.11.0" // IP Packets Discarded (output)
	OIDIpInErrors    = "1.3.6.1.2.1.4.5.0"  // IP Input Errors
	OIDIpOutErrors   = "1.3.6.1.2.1.4.12.0" // IP Output Errors

	// === ICMP Statistics (ICMP-MIB) ===
	OIDIcmpInEchoReps = "1.3.6.1.2.1.5.21.0" // ICMP Echo Replies
	OIDIcmpOutEchos   = "1.3.6.1.2.1.5.19.0" // ICMP Echo Requests

	// === TCP Statistics (TCP-MIB) ===
	OIDTcpCurrEstab = "1.3.6.1.2.1.6.9.0"  // TCP Established Connections
	OIDTcpInSegs    = "1.3.6.1.2.1.6.10.0" // TCP Segments In
	OIDTcpOutSegs   = "1.3.6.1.2.1.6.11.0" // TCP Segments Out

	// === UDP Statistics (UDP-MIB) ===
	OIDUdpInDatagrams  = "1.3.6.1.2.1.7.1.0" // UDP Datagrams In
	OIDUdpOutDatagrams = "1.3.6.1.2.1.7.4.0" // UDP Datagrams Out
	OIDUdpInErrors     = "1.3.6.1.2.1.7.3.0" // UDP Input Errors
	OIDUdpNoPorts      = "1.3.6.1.2.1.7.2.0" // UDP No Port Errors
)

// StatusValue for Interface Status
const (
	StatusUp      = 1
	StatusDown    = 2
	StatusTesting = 3
)

// StatusString returns string representation of status
func StatusString(status int) string {
	switch status {
	case StatusUp:
		return "up"
	case StatusDown:
		return "down"
	case StatusTesting:
		return "testing"
	default:
		return "unknown"
	}
}

// InterfaceType constants
const (
	IfTypeEthernet   = 6
	IfTypeTokenRing  = 9
	IfTypePPP        = 23
	IfTypeLoopback   = 24
	IfTypeAtoM       = 37
	IfTypeFrameRelay = 32
)

// GetInterfaceTypeName returns interface type name
func GetInterfaceTypeName(ifType int) string {
	switch ifType {
	case IfTypeEthernet:
		return "ethernet"
	case IfTypeTokenRing:
		return "token-ring"
	case IfTypePPP:
		return "ppp"
	case IfTypeLoopback:
		return "loopback"
	case IfTypeAtoM:
		return "atm"
	case IfTypeFrameRelay:
		return "frame-relay"
	default:
		return "unknown"
	}
}
