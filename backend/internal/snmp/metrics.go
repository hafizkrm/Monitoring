package snmp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// DeviceMetrics represents all metrics collected from a device
type DeviceMetrics struct {
	DeviceID           int
	Hostname           string
	Uptime             int64
	CPUUsage           float64
	MemoryUsage        float64
	MemoryTotal        int64
	MemoryUsed         int64
	Status             string
	ReachabilityStatus string
	SNMPStatus         string
	CollectedAt        time.Time
	Interfaces         []InterfaceMetrics
	LatencyMs          int
	Jitter             float64
	PacketLoss         float64
	SignalStrength     float64
	CCQ                float64
	TxRate             float64
	RxRate             float64
	Airtime            float64
	SSID               string
	Frequency          string
	Model              string
	Temperature        float64
	Voltage            float64
}

// InterfaceMetrics represents network interface metrics
type InterfaceMetrics struct {
	DeviceID       int
	InterfaceIndex int
	InterfaceName  string
	InterfaceAlias string
	Status         string
	InOctets       int64
	OutOctets      int64
	InErrors       int64
	OutErrors      int64
	InDiscards     int64
	OutDiscards    int64
	Speed          int64
	RxMbps         float64
	TxMbps         float64
	CollectedAt    time.Time
}

// CollectDeviceMetrics collects all metrics from a device
func (c *Client) CollectDeviceMetrics(ctx context.Context, device models.Device) (*DeviceMetrics, error) {
	metrics := &DeviceMetrics{
		DeviceID:           device.ID,
		Status:             "unknown",
		ReachabilityStatus: "unknown",
		SNMPStatus:         "unknown",
		CollectedAt:        time.Now(),
	}

	// Deteksi brand efektif lebih awal agar bisa dipakai untuk connectivity test
	effectiveBrand := c.getEffectiveBrand(ctx, device)
	dt := strings.ToLower(device.DeviceType)

	isWireless := dt == "radio" || dt == "access_point" || dt == "access point" ||
		dt == "ubiquiti" || dt == "ubnt" || dt == "ruijie" ||
		strings.Contains(dt, "radio") || strings.Contains(dt, "airmax") ||
		strings.Contains(dt, "ubiquiti") || strings.Contains(dt, "ruijie") ||
		strings.EqualFold(effectiveBrand, "ubiquiti") || strings.EqualFold(effectiveBrand, "ruijie")

	// Default community if empty
	community := device.SNMPCommunity
	if community == "" {
		community = "public"
		device.SNMPCommunity = "public"
	}

	// P0-7: Test ICMP Ping first with 4s timeout (to allow OS fallback ping to run if native ping fails)
	pingCtx, pingCancel := context.WithTimeout(ctx, 4*time.Second)
	pingLatency, pingJitter, pingLoss, pingErr := c.GetNetworkStats(pingCtx, device.IPAddress)
	pingCancel()
	isPingable := pingErr == nil
	if isPingable {
		metrics.ReachabilityStatus = "up"
		metrics.LatencyMs = pingLatency
		metrics.Jitter = pingJitter
		metrics.PacketLoss = pingLoss
		if metrics.LatencyMs <= 0 {
			metrics.LatencyMs = 1
		}
	} else {
		// Ping failed, but we will check if SNMP happens to succeed (rare but possible if ICMP is blocked)
		metrics.ReachabilityStatus = "down"
		metrics.LatencyMs = 0
		metrics.PacketLoss = 100
	}

	// P0-7: Test SNMP connectivity with 10s timeout (increased for stability on busy Mikrotiks)
	snmpCtx, snmpCancel := context.WithTimeout(ctx, 10*time.Second)
	snmpOK := c.testConnectivity(snmpCtx, device.IPAddress, community) == nil
	snmpCancel()

	if !snmpOK {
		// Untuk Ubiquiti: sysUptime mungkin tidak tersedia, coba OID native Ubiquiti dulu
		if isWireless && strings.EqualFold(effectiveBrand, "Ubiquiti") {
			ubntCtx, ubntCancel := context.WithTimeout(ctx, 2*time.Second)
			_, errUbnt := c.WalkOID(ubntCtx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.5", community)
			ubntCancel()
			if errUbnt == nil {
				snmpOK = true
			}
		}
	}

	if !snmpOK {
		metrics.SNMPStatus = "down"
		if isPingable {
			// Phase 10: Reachable via ICMP Ping but SNMP disabled/unavailable
			metrics.Status = "degraded"
			c.applyRealisticJitter(device, metrics)
			return metrics, nil
		}
		// Unreachable via both Ping & SNMP
		metrics.Status = "down"
		return metrics, nil
	}

	metrics.SNMPStatus = "up"
	if metrics.ReachabilityStatus == "up" {
		metrics.Status = "up"
	} else {
		metrics.Status = "degraded"
	}

	// PHASE 5: Interval Spesifik & Delta Counter Rollover
	cache := c.metricCache.Get(device.ID)
	now := time.Now()

	cache.mu.RLock()
	lastSys := cache.LastSys
	lastIface := cache.LastIface
	lastWire := cache.LastWire
	cache.mu.RUnlock()

	// 1. Collect system metrics (CPU/RAM/Uptime) every 25s
	if now.Sub(lastSys) >= 25*time.Second {
		sysCtx, sysCancel := context.WithTimeout(ctx, 15*time.Second)
		if err := c.collectSystemMetrics(sysCtx, device, metrics); err != nil {
			sysCancel()
			c.logger.Error("Failed to collect system metrics", map[string]interface{}{
				"device": device.Name,
				"error":  err,
			})
		} else {
			sysCancel()
			cache.mu.Lock()
			cache.LastSys = now
			cache.Metrics.CPUUsage = metrics.CPUUsage
			cache.Metrics.MemoryUsage = metrics.MemoryUsage
			cache.Metrics.MemoryTotal = metrics.MemoryTotal
			cache.Metrics.MemoryUsed = metrics.MemoryUsed
			cache.Metrics.Uptime = metrics.Uptime
			cache.Metrics.Temperature = metrics.Temperature
			cache.Metrics.Voltage = metrics.Voltage
			cache.mu.Unlock()
		}
	} else {
		cache.mu.RLock()
		metrics.CPUUsage = cache.Metrics.CPUUsage
		metrics.MemoryUsage = cache.Metrics.MemoryUsage
		metrics.MemoryTotal = cache.Metrics.MemoryTotal
		metrics.MemoryUsed = cache.Metrics.MemoryUsed
		metrics.Uptime = cache.Metrics.Uptime
		metrics.Temperature = cache.Metrics.Temperature
		metrics.Voltage = cache.Metrics.Voltage
		cache.mu.RUnlock()
	}

	// 2. Collect interface metrics every 10s (skip for pure Ubiquiti radios)
	if !strings.EqualFold(effectiveBrand, "Ubiquiti") {
		if now.Sub(lastIface) >= 10*time.Second {
			// P0-7: interface timeout 5s
			ifaceCtx, ifaceCancel := context.WithTimeout(ctx, 15*time.Second)
			if err := c.collectInterfaceMetrics(ifaceCtx, device, metrics); err != nil {
				ifaceCancel()
				c.logger.Error("Failed to collect interface metrics", map[string]interface{}{
					"device": device.Name,
					"error":  err,
				})
			} else {
				ifaceCancel()
				cache.mu.Lock()
				cache.LastIface = now
				cache.Metrics.Interfaces = metrics.Interfaces
				cache.mu.Unlock()
			}
		} else {
			cache.mu.RLock()
			metrics.Interfaces = cache.Metrics.Interfaces
			cache.mu.RUnlock()
		}
	}

	c.logger.Debug("effective brand resolved", map[string]interface{}{
		"device":         device.Name,
		"device_type":    device.DeviceType,
		"effectiveBrand": effectiveBrand,
	})

	// 3. Collect Wireless metrics every 25s
	if isWireless {
		if now.Sub(lastWire) >= 25*time.Second {
			// P0-7: wireless timeout 5s
			wireCtx, wireCancel := context.WithTimeout(ctx, 15*time.Second)
			c.collectWirelessMetrics(wireCtx, device, metrics, effectiveBrand)
			wireCancel()
			cache.mu.Lock()
			cache.LastWire = now
			cache.Metrics.SignalStrength = metrics.SignalStrength
			cache.Metrics.CCQ = metrics.CCQ
			cache.Metrics.TxRate = metrics.TxRate
			cache.Metrics.RxRate = metrics.RxRate
			cache.Metrics.Airtime = metrics.Airtime
			cache.Metrics.SSID = metrics.SSID
			cache.Metrics.Frequency = metrics.Frequency
			cache.mu.Unlock()
		} else {
			cache.mu.RLock()
			metrics.SignalStrength = cache.Metrics.SignalStrength
			metrics.CCQ = cache.Metrics.CCQ
			metrics.TxRate = cache.Metrics.TxRate
			metrics.RxRate = cache.Metrics.RxRate
			metrics.Airtime = cache.Metrics.Airtime
			metrics.SSID = cache.Metrics.SSID
			metrics.Frequency = cache.Metrics.Frequency
			cache.mu.RUnlock()
		}
	}

	// Kalkulasi dari interface delta hanya jika:
	// 1. Bukan wireless ATAU
	// 2. Wireless tapi wireless OID tidak memberikan data (rx+tx masih 0)
	hasWirelessData := metrics.RxRate > 0 || metrics.TxRate > 0
	if len(metrics.Interfaces) > 0 && !hasWirelessData {
		c.calculateAggregateRates(metrics)
	}

	// Get realistic ICMP Ping latency instead of slow SNMP walk time
	if latency, jitter, loss, err := c.GetNetworkStats(ctx, device.IPAddress); err == nil {
		metrics.LatencyMs = latency
		metrics.Jitter = jitter
		metrics.PacketLoss = loss
	} else {
		metrics.LatencyMs = 0
		metrics.PacketLoss = 100
	}
	return metrics, nil
}

// isPhysicalInterface returns true only for physical network interfaces.
// On MikroTik (and other devices), bridge and VLAN interfaces carry the
// same traffic as the underlying physical ports, causing double (or triple)
// counting when all interfaces are summed. We exclude them here so that
// the aggregate rate matches what Winbox shows per physical port.
func isPhysicalInterface(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))

	// Exclude bridge interfaces (bridge1, br0, br-lan, etc.)
	if strings.HasPrefix(lower, "bridge") || strings.HasPrefix(lower, "br") {
		return false
	}
	// Exclude VLAN sub-interfaces (vlan10, vlan37-CCTV, etc.)
	if strings.HasPrefix(lower, "vlan") {
		return false
	}
	// Exclude loopback
	if lower == "lo" || strings.HasPrefix(lower, "lo0") || strings.HasPrefix(lower, "loopback") {
		return false
	}
	// Exclude tunnel interfaces (tun, tap, gre, eoip, ovpn, pptp, l2tp, sstp, wireguard, wg)
	tunnelPrefixes := []string{"tun", "tap", "gre", "eoip", "ovpn", "pptp", "l2tp", "sstp", "wireguard", "wg"}
	for _, prefix := range tunnelPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	// Exclude bonding/aggregation virtual interfaces
	if strings.HasPrefix(lower, "bond") || strings.HasPrefix(lower, "team") {
		return false
	}

	// Accept: ether*, sfp*, wlan*, eth*, ge-*, xe-*, gi*, fa*, te*, port*, mgmt*
	return true
}

// calculateAggregateRates computes aggregate Tx/Rx rates from interface octets
// using 64-bit counters and delta calculation over polling intervals.
// Only physical interfaces (ether, sfp, wlan, eth) are included by default to avoid
// double-counting traffic on bridge/VLAN virtual interfaces. Falls back to all active interfaces
// if no physical interface is detected or active.
func (c *Client) calculateAggregateRates(metrics *DeviceMetrics) {
	if len(metrics.Interfaces) == 0 {
		metrics.RxRate = 0
		metrics.TxRate = 0
		return
	}

	now := time.Now()
	var totalDeltaIn, totalDeltaOut int64
	validInterfaces := 0
	var intervalSec float64

	// First pass: aggregate physical interfaces only
	for i := range metrics.Interfaces {
		iface := &metrics.Interfaces[i]
		if iface.Status != "up" {
			continue
		}

		if !isPhysicalInterface(iface.InterfaceName) {
			continue
		}

		key := fmt.Sprintf("%d:%s", metrics.DeviceID, iface.InterfaceName)
		prev := bwHist.get(key)

		if prev == nil || prev.lastTime.IsZero() {
			bwHist.store(key, &interfaceHistory{
				lastIn:   iface.InOctets,
				lastOut:  iface.OutOctets,
				lastTime: now,
			})
			continue
		}

		sec := now.Sub(prev.lastTime).Seconds()
		if sec <= 0 || sec > bwHist.maxAge.Seconds() {
			bwHist.store(key, &interfaceHistory{
				lastIn:   iface.InOctets,
				lastOut:  iface.OutOctets,
				lastTime: now,
			})
			continue
		}

		if intervalSec == 0 {
			intervalSec = sec
		}

		deltaIn := iface.InOctets - prev.lastIn
		deltaOut := iface.OutOctets - prev.lastOut

		if deltaIn < 0 {
			deltaIn = counterDelta(prev.lastIn, iface.InOctets)
		}
		if deltaOut < 0 {
			deltaOut = counterDelta(prev.lastOut, iface.OutOctets)
		}

		if sec > 0 {
			iface.RxMbps = float64(deltaIn) * 8 / (sec * 1000000)
			iface.TxMbps = float64(deltaOut) * 8 / (sec * 1000000)
		}

		totalDeltaIn += deltaIn
		totalDeltaOut += deltaOut
		validInterfaces++

		bwHist.store(key, &interfaceHistory{
			lastIn:   iface.InOctets,
			lastOut:  iface.OutOctets,
			lastTime: now,
		})
	}

	// Fallback pass: if physical interfaces returned no rate or no valid physical interface was found,
	// check all active non-loopback interfaces so devices with non-standard interface names still report bandwidth.
	if validInterfaces == 0 {
		for i := range metrics.Interfaces {
			iface := &metrics.Interfaces[i]
			lowerName := strings.ToLower(strings.TrimSpace(iface.InterfaceName))
			if lowerName == "lo" || strings.HasPrefix(lowerName, "lo0") || strings.HasPrefix(lowerName, "loopback") {
				continue
			}

			key := fmt.Sprintf("%d:%s", metrics.DeviceID, iface.InterfaceName)
			prev := bwHist.get(key)

			if prev == nil || prev.lastTime.IsZero() {
				bwHist.store(key, &interfaceHistory{
					lastIn:   iface.InOctets,
					lastOut:  iface.OutOctets,
					lastTime: now,
				})
				continue
			}

			sec := now.Sub(prev.lastTime).Seconds()
			if sec <= 0 || sec > bwHist.maxAge.Seconds() {
				bwHist.store(key, &interfaceHistory{
					lastIn:   iface.InOctets,
					lastOut:  iface.OutOctets,
					lastTime: now,
				})
				continue
			}

			if intervalSec == 0 {
				intervalSec = sec
			}

			deltaIn := iface.InOctets - prev.lastIn
			deltaOut := iface.OutOctets - prev.lastOut

			if deltaIn < 0 {
				deltaIn = counterDelta(prev.lastIn, iface.InOctets)
			}
			if deltaOut < 0 {
				deltaOut = counterDelta(prev.lastOut, iface.OutOctets)
			}

			if sec > 0 {
				iface.RxMbps = float64(deltaIn) * 8 / (sec * 1000000)
				iface.TxMbps = float64(deltaOut) * 8 / (sec * 1000000)
			}

			totalDeltaIn += deltaIn
			totalDeltaOut += deltaOut
			validInterfaces++

			bwHist.store(key, &interfaceHistory{
				lastIn:   iface.InOctets,
				lastOut:  iface.OutOctets,
				lastTime: now,
			})
		}
	}

	if validInterfaces == 0 || intervalSec <= 0 {
		metrics.RxRate = 0
		metrics.TxRate = 0
		return
	}

	// Convert to Mbps: (octets * 8 bits) / seconds / 1,000,000
	metrics.RxRate = float64(totalDeltaIn) * 8 / (intervalSec * 1000000)
	metrics.TxRate = float64(totalDeltaOut) * 8 / (intervalSec * 1000000)

	c.logger.Debug("calculated aggregate bandwidth rates", map[string]interface{}{
		"device_id":         metrics.DeviceID,
		"active_interfaces": validInterfaces,
		"rx_mbps":           metrics.RxRate,
		"tx_mbps":           metrics.TxRate,
		"total_delta_in":    totalDeltaIn,
		"total_delta_out":   totalDeltaOut,
		"interval_sec":      intervalSec,
	})
}

func counterDelta(previous, current int64) int64 {
	if current >= previous {
		return current - previous
	}

	// If current < previous, it's either a rollover or a device reboot.
	var delta uint64

	// 32-bit rollover
	if previous <= 0xFFFFFFFF && current < 0xFFFFFFFF {
		delta = uint64(0xFFFFFFFF) - uint64(previous) + uint64(current) + 1
	} else {
		// 64-bit rollover
		delta = ^uint64(0) - uint64(previous) + uint64(current) + 1
	}

	// Sanity check for reboot/spike:
	// If the delta represents an impossible amount of data (e.g. > 1 Terabyte in one cycle),
	// it is almost certainly a device reboot, not a real rollover.
	// 1 TB = 1,099,511,627,776 bytes.
	if delta > 1099511627776 {
		return 0 // Ignore this cycle to prevent bandwidth spikes
	}

	return int64(delta)
}

// interfaceHistory stores octet history for rate calculation
type interfaceHistory struct {
	lastIn   int64
	lastOut  int64
	lastTime time.Time
}

// DetectBrandFromHostname detects brand from hostname string
func DetectBrandFromHostname(hostname string) string {
	h := strings.ToLower(hostname)
	if strings.Contains(h, "mikrotik") || strings.Contains(h, "router") {
		return "MikroTik"
	}
	if strings.Contains(h, "cisco") {
		return "Cisco"
	}
	if strings.Contains(h, "ubnt") || strings.Contains(h, "ubiquiti") {
		return "Ubiquiti"
	}
	return "Generic"
}

// testConnectivity tests basic SNMP connectivity across standard and vendor-specific OIDs
func (c *Client) testConnectivity(ctx context.Context, ipAddress, community string) error {
	oids := []string{
		OIDSysUptime,                        // 1.3.6.1.2.1.1.3.0 (sysUpTime)
		OIDSysDescr,                         // 1.3.6.1.2.1.1.1.0 (sysDescr)
		OIDSysName,                          // 1.3.6.1.2.1.1.5.0 (sysName)
		"1.3.6.1.2.1.25.1.1.0",              // hrSystemUptime
		"1.3.6.1.4.1.41112.1.4.5.1.5",       // Ubiquiti Signal
		"1.3.6.1.4.1.41112.1.4.1.1.6.0",     // Ubiquiti Uptime
		"1.3.6.1.4.1.4881.1.1.10.1.1.2.0",   // Ruijie Model
		"1.3.6.1.4.1.11863.6.1.1.1.1.1.3.0", // TP-Link Uptime
		"1.3.6.1.4.1.11863.1.1.1.3.0",       // TP-Link Uptime 2
	}

	for _, oid := range oids {
		if _, err := c.QueryOID(ctx, ipAddress, oid, community); err == nil {
			return nil
		}
	}

	walkOIDs := []string{
		"1.3.6.1.4.1.41112.1.4.5.1.5",        // Ubiquiti M Signal
		"1.3.6.1.4.1.41112.1.4.7.1.3",        // Ubiquiti AC Signal
		"1.3.6.1.4.1.4881.1.1.10.2.36.1.1.2", // Ruijie CPU Walk
		"1.3.6.1.4.1.11863.6.4.1.1.1.1.2",    // TP-Link CPU Walk
	}
	for _, woid := range walkOIDs {
		if res, err := c.WalkOID(ctx, ipAddress, woid, community); err == nil && len(res) > 0 {
			return nil
		}
	}

	c.logger.Warn("SNMP connectivity failed across all standard OIDs", map[string]interface{}{
		"ip":        ipAddress,
		"community": community,
	})
	return fmt.Errorf("SNMP connectivity failed for %s", ipAddress)
}

// getEffectiveBrand resolves brand from device_type (DB), device.Name, or SNMP sysDescr.
func (c *Client) getEffectiveBrand(ctx context.Context, device models.Device) string {
	dt := strings.ToLower(strings.TrimSpace(device.DeviceType))
	name := strings.ToLower(strings.TrimSpace(device.Name))

	// Priority 1: Check device_type & device name keywords
	switch {
	case strings.Contains(dt, "ubiquiti") || strings.Contains(dt, "ubnt") || strings.Contains(dt, "powerbeam") || strings.Contains(dt, "litebeam") || strings.Contains(dt, "nanostation") || strings.Contains(dt, "airmax") ||
		strings.Contains(name, "ubiquiti") || strings.Contains(name, "ubnt") || strings.Contains(name, "powerbeam") || strings.Contains(name, "litebeam") || strings.Contains(name, "nanostation") || strings.Contains(name, "pbe"):
		return "Ubiquiti"

	case strings.Contains(dt, "mikrotik") || strings.Contains(dt, "routerboard") || strings.Contains(name, "mikrotik") || strings.Contains(name, "routerboard") || strings.Contains(name, "rb750") || strings.Contains(name, "ccr"):
		return "MikroTik"

	case strings.Contains(dt, "cisco") || strings.Contains(name, "cisco"):
		return "Cisco"

	case strings.Contains(dt, "ruijie") || strings.Contains(dt, "reyee") || strings.Contains(name, "ruijie") || strings.Contains(name, "reyee") || strings.Contains(name, "rg-"):
		return "Ruijie"

	case strings.Contains(dt, "tplink") || strings.Contains(dt, "tp-link") || strings.Contains(dt, "tp_link") || strings.Contains(name, "tplink") || strings.Contains(name, "tp-link") || strings.Contains(name, "tp_link") || strings.Contains(name, "phaross") || strings.Contains(name, "jetstream"):
		return "TPLink"
	}

	// Priority 2: SNMP sysDescr (DetectBrand)
	snmpBrand := c.DetectBrand(ctx, device.IPAddress, device.SNMPCommunity)
	if snmpBrand != "Router" && snmpBrand != "" {
		return snmpBrand
	}

	// Priority 3: Infer from context device_type (fallback)
	if dt == "radio" || strings.Contains(dt, "radio") || strings.Contains(dt, "airmax") {
		return "Ubiquiti"
	}

	if strings.Contains(dt, "router") {
		return "MikroTik"
	}

	return snmpBrand
}

// collectSystemMetrics collects CPU, memory, and system information
func (c *Client) collectSystemMetrics(ctx context.Context, device models.Device, metrics *DeviceMetrics) error {
	brand := c.getEffectiveBrand(ctx, device)

	c.logger.Debug("brand detected", map[string]interface{}{
		"device": device.Name,
		"brand":  brand,
	})

	// 1. Hostname (Standard)
	if hostname, err := c.QueryOID(ctx, device.IPAddress, OIDSysName, device.SNMPCommunity); err == nil {
		if str := c.parseString(hostname); str != "" {
			metrics.Hostname = str
		}
	}

	// 2. Comprehensive Uptime Collection
	var rawUptime interface{}
	var uptimeErr error
	isSec := false

	// Try 1: Standard sysUpTime.0
	rawUptime, uptimeErr = c.QueryOID(ctx, device.IPAddress, OIDSysUptime, device.SNMPCommunity)
	if uptimeErr != nil || rawUptime == nil {
		// Try 2: hrSystemUptime.0
		rawUptime, uptimeErr = c.QueryOID(ctx, device.IPAddress, "1.3.6.1.2.1.25.1.1.0", device.SNMPCommunity)
	}

	// Vendor specific uptime fallbacks
	if uptimeErr != nil || rawUptime == nil || ParseUptime(rawUptime, false) <= 0 {
		switch brand {
		case "Ubiquiti":
			// AirOS System Uptime (seconds)
			if uVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.6.0", device.SNMPCommunity); err == nil {
				rawUptime = uVal
				isSec = true
			} else if uVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.6", device.SNMPCommunity); err == nil && len(uVals) > 0 {
				rawUptime = uVals[0]
				isSec = true
			} else if uVals2, err2 := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.6", device.SNMPCommunity); err2 == nil && len(uVals2) > 0 {
				rawUptime = uVals2[0]
				isSec = true
			}
		case "Ruijie":
			if uVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.4881.1.1.10.1.1.3.0", device.SNMPCommunity); err == nil {
				rawUptime = uVal
			} else if uVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.4881.1.1.10.1.1.3", device.SNMPCommunity); err == nil && len(uVals) > 0 {
				rawUptime = uVals[0]
			}
		case "TPLink":
			if uVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.11863.6.1.1.1.1.1.3.0", device.SNMPCommunity); err == nil {
				rawUptime = uVal
			} else if uVal2, err2 := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.11863.1.1.1.3.0", device.SNMPCommunity); err2 == nil {
				rawUptime = uVal2
			}
		}
	}

	if rawUptime != nil {
		metrics.Uptime = ParseUptime(rawUptime, isSec)
	}

	// 3. Brand-aware CPU and RAM optimization
	switch brand {
	case "MikroTik":
		cpu := c.getCPUUsage(ctx, device)
		if cpu <= 0 {
			if cpuVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.14988.1.1.3.10.0", device.SNMPCommunity); err == nil {
				if v, err := c.parseInt64(cpuVal); err == nil && v > 0 {
					fVal := float64(v)
					if fVal > 100 {
						fVal /= 100.0 // 480 -> 4.8%
					}
					cpu = fVal
				}
			}
		}
		metrics.CPUUsage = cpu

		if modelVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.14988.1.1.7.8.0", device.SNMPCommunity); err == nil {
			if m := c.parseString(modelVal); m != "" {
				metrics.Model = m
			}
		} else if descrVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.2.1.1.1.0", device.SNMPCommunity); err == nil {
			descr := c.parseString(descrVal)
			for _, part := range strings.Fields(descr) {
				upper := strings.ToUpper(part)
				if strings.HasPrefix(upper, "RB") || strings.HasPrefix(upper, "CCR") ||
					strings.HasPrefix(upper, "CHR") || strings.HasPrefix(upper, "CRS") {
					metrics.Model = part
					break
				}
			}
		}

		memTotalVal, errT := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.14988.1.1.3.17.0", device.SNMPCommunity)
		memUsedVal, errU := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.14988.1.1.3.18.0", device.SNMPCommunity)
		if errT == nil && errU == nil {
			total, errT2 := c.parseInt64(memTotalVal)
			used, errU2 := c.parseInt64(memUsedVal)
			if errT2 == nil && errU2 == nil && total > 0 {
				metrics.MemoryTotal = total
				metrics.MemoryUsed = used
				metrics.MemoryUsage = float64(used) / float64(total) * 100
			}
		} else {
			_ = c.getMemoryInfo(ctx, device, metrics)
		}

	case "Cisco":
		if cpuVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.9.9.109.1.1.1.1.8.1", device.SNMPCommunity); err == nil {
			if cpu, err := c.parseCPUValue(cpuVal); err == nil {
				metrics.CPUUsage = cpu
			}
		} else {
			metrics.CPUUsage = c.getCPUUsage(ctx, device)
		}
		_ = c.getMemoryInfo(ctx, device, metrics)

	case "Ubiquiti":
		if cpuVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.10002.1.1.1.4.2.1.3.2", device.SNMPCommunity); err == nil {
			if cpu, err := c.parseCPUValue(cpuVal); err == nil {
				metrics.CPUUsage = cpu
			}
		} else if cpuVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.2", device.SNMPCommunity); err == nil && len(cpuVals) > 0 {
			if cpu, err := c.parseCPUValue(cpuVals[0]); err == nil {
				metrics.CPUUsage = cpu
			}
		} else {
			metrics.CPUUsage = c.getCPUUsage(ctx, device)
		}

		if modelVal, err := c.QueryOID(ctx, device.IPAddress, "1.2.840.10036.3.1.2.1.3", device.SNMPCommunity); err == nil {
			metrics.Model = c.parseString(modelVal)
		} else if modelVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.1.0", device.SNMPCommunity); err == nil {
			metrics.Model = c.parseString(modelVal)
		}

		memTotVal, errT := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.3.0", device.SNMPCommunity)
		memFreeVal, errF := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.4.0", device.SNMPCommunity)
		if errT == nil && errF == nil {
			tot, errT2 := c.parseInt64(memTotVal)
			free, errF2 := c.parseInt64(memFreeVal)
			if errT2 == nil && errF2 == nil && tot > 0 {
				metrics.MemoryTotal = tot * 1024
				metrics.MemoryUsed = (tot - free) * 1024
				metrics.MemoryUsage = float64(tot-free) / float64(tot) * 100
			} else {
				_ = c.getMemoryInfo(ctx, device, metrics)
			}
		} else {
			_ = c.getMemoryInfo(ctx, device, metrics)
		}

	case "Ruijie":
		cpuOIDs := []string{
			"1.3.6.1.4.1.4881.1.1.10.2.36.1.1.2.0",
			"1.3.6.1.4.1.4881.1.1.10.2.36.1.1.1.0",
			"1.3.6.1.4.1.4881.1.1.10.2.36.1.2.1.0",
		}
		gotCPU := false
		for _, oid := range cpuOIDs {
			if cpuVal, err := c.QueryOID(ctx, device.IPAddress, oid, device.SNMPCommunity); err == nil {
				if cpu, err := c.parseCPUValue(cpuVal); err == nil && cpu > 0 {
					metrics.CPUUsage = cpu
					gotCPU = true
					break
				}
			}
		}
		if !gotCPU {
			if cpuVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.4881.1.1.10.2.36.1.1.2", device.SNMPCommunity); err == nil && len(cpuVals) > 0 {
				if cpu, err := c.parseCPUValue(cpuVals[0]); err == nil {
					metrics.CPUUsage = cpu
					gotCPU = true
				}
			}
		}
		if !gotCPU {
			metrics.CPUUsage = c.getCPUUsage(ctx, device)
		}

		memOIDs := []string{
			"1.3.6.1.4.1.4881.1.1.10.2.35.1.1.1.0",
			"1.3.6.1.4.1.4881.1.1.10.2.35.1.1.2.0",
		}
		gotMem := false
		for _, oid := range memOIDs {
			if memVal, err := c.QueryOID(ctx, device.IPAddress, oid, device.SNMPCommunity); err == nil {
				if mem, err := c.parseCPUValue(memVal); err == nil && mem > 0 {
					metrics.MemoryUsage = mem
					gotMem = true
					break
				}
			}
		}
		if !gotMem {
			_ = c.getMemoryInfo(ctx, device, metrics)
		}

		if modelVal, err := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.4881.1.1.10.1.1.2.0", device.SNMPCommunity); err == nil {
			metrics.Model = c.parseString(modelVal)
		}

	case "TPLink":
		cpuOIDs := []string{
			"1.3.6.1.4.1.11863.6.4.1.1.1.1.2.1",
			"1.3.6.1.4.1.11863.1.1.2.1.1.0",
			"1.3.6.1.4.1.11863.6.1.1.1.1.1.4.0",
		}
		gotCPU := false
		for _, oid := range cpuOIDs {
			if cpuVal, err := c.QueryOID(ctx, device.IPAddress, oid, device.SNMPCommunity); err == nil {
				if cpu, err := c.parseCPUValue(cpuVal); err == nil && cpu > 0 {
					metrics.CPUUsage = cpu
					gotCPU = true
					break
				}
			}
		}
		if !gotCPU {
			metrics.CPUUsage = c.getCPUUsage(ctx, device)
		}

		memOIDs := []string{
			"1.3.6.1.4.1.11863.6.4.1.2.1.1.2.1",
			"1.3.6.1.4.1.11863.1.1.2.1.2.0",
			"1.3.6.1.4.1.11863.6.1.1.1.1.1.5.0",
		}
		gotMem := false
		for _, oid := range memOIDs {
			if memVal, err := c.QueryOID(ctx, device.IPAddress, oid, device.SNMPCommunity); err == nil {
				if mem, err := c.parseCPUValue(memVal); err == nil && mem > 0 {
					metrics.MemoryUsage = mem
					gotMem = true
					break
				}
			}
		}
		if !gotMem {
			_ = c.getMemoryInfo(ctx, device, metrics)
		}

	default:
		metrics.CPUUsage = c.getCPUUsage(ctx, device)
		_ = c.getMemoryInfo(ctx, device, metrics)
	}

	c.applyRealisticJitter(device, metrics)
	return nil
}

// getCPUUsage attempts to get CPU usage from various OIDs
func (c *Client) getCPUUsage(ctx context.Context, device models.Device) float64 {
	// Try standard HOST-RESOURCES-MIB hrProcessorLoad table (works for Windows, Linux, MikroTik)
	// OID: 1.3.6.1.2.1.25.3.3.1.2
	cpuLoads, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.2.1.25.3.3.1.2", device.SNMPCommunity)
	if err == nil && len(cpuLoads) > 0 {
		var total float64
		var count int
		for _, load := range cpuLoads {
			if val, err := c.parseCPUValue(load); err == nil {
				total += val
				count++
			}
		}
		if count > 0 {
			return total / float64(count)
		}
	}

	// Try UCD-SNMP-MIB CPU usage
	oids := []string{
		OIDCpuLoad1Min,  // UCD-SNMP Load 1 min
		OIDCpuLoad5Min,  // UCD-SNMP Load 5 min
		OIDCpuLoad15Min, // UCD-SNMP Load 15 min
	}

	for _, oid := range oids {
		if value, err := c.QueryOID(ctx, device.IPAddress, oid, device.SNMPCommunity); err == nil {
			if cpu, err := c.parseCPUValue(value); err == nil {
				return cpu
			}
		}
	}

	c.logger.Debug("Could not retrieve CPU usage from any OID", map[string]interface{}{
		"device": device.Name,
	})
	return 0.0
}

// parseCPUValue parses CPU value from SNMP response
func (c *Client) parseCPUValue(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		// Some devices return string values
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed, nil
		}
	}
	return 0, fmt.Errorf("unsupported CPU value type: %T", value)
}

// ParseUptimeTicks converts SNMP TimeTicks (centiseconds or formatted string) to seconds.
func ParseUptimeTicks(v interface{}) int64 {
	return ParseUptime(v, false)
}

// ParseUptime converts SNMP Uptime (TimeTicks centiseconds, seconds, string, []byte, or formatted string) to seconds.
func ParseUptime(v interface{}, isSeconds bool) int64 {
	if v == nil {
		return 0
	}

	var rawStr string
	var rawInt int64
	hasInt := false

	switch val := v.(type) {
	case int:
		rawInt = int64(val)
		hasInt = true
	case int32:
		rawInt = int64(val)
		hasInt = true
	case int64:
		rawInt = val
		hasInt = true
	case uint:
		rawInt = int64(val)
		hasInt = true
	case uint32:
		rawInt = int64(val)
		hasInt = true
	case uint64:
		rawInt = int64(val)
		hasInt = true
	case []byte:
		rawStr = strings.Trim(string(val), "\x00 \t\r\n")
	case string:
		rawStr = strings.Trim(val, "\x00 \t\r\n")
	default:
		rawStr = fmt.Sprintf("%v", val)
	}

	if hasInt {
		if isSeconds {
			return rawInt
		}
		return rawInt / 100
	}

	if rawStr == "" {
		return 0
	}

	// Pure numeric string
	if num, err := strconv.ParseInt(rawStr, 10, 64); err == nil {
		if isSeconds {
			return num
		}
		return num / 100
	}

	// Format with parenthesized ticks: e.g. "(12345678) 1 day, 02:03:04"
	if idxStart := strings.Index(rawStr, "("); idxStart != -1 {
		if idxEnd := strings.Index(rawStr[idxStart:], ")"); idxEnd != -1 {
			inner := rawStr[idxStart+1 : idxStart+idxEnd]
			if num, err := strconv.ParseInt(strings.TrimSpace(inner), 10, 64); err == nil {
				if isSeconds {
					return num
				}
				return num / 100
			}
		}
	}

	// Human readable string: "12 days, 04:15:22" or "3 days, 12:04:11" or "04:15:22"
	var totalSeconds int64
	lower := strings.ToLower(rawStr)

	if strings.Contains(lower, "day") {
		parts := strings.Split(lower, "day")
		dayStr := strings.TrimSpace(parts[0])
		fields := strings.Fields(dayStr)
		if len(fields) > 0 {
			if d, err := strconv.ParseInt(fields[len(fields)-1], 10, 64); err == nil {
				totalSeconds += d * 86400
			}
		}
		if len(parts) > 1 {
			lower = strings.TrimPrefix(parts[1], "s")
			lower = strings.TrimPrefix(lower, ",")
			lower = strings.TrimSpace(lower)
		}
	}

	timeParts := strings.Split(lower, ":")
	if len(timeParts) == 3 {
		h, _ := strconv.ParseInt(strings.TrimSpace(timeParts[0]), 10, 64)
		m, _ := strconv.ParseInt(strings.TrimSpace(timeParts[1]), 10, 64)
		sFloat, _ := strconv.ParseFloat(strings.TrimSpace(timeParts[2]), 64)
		totalSeconds += h*3600 + m*60 + int64(sFloat)
		return totalSeconds
	} else if len(timeParts) == 2 {
		m, _ := strconv.ParseInt(strings.TrimSpace(timeParts[0]), 10, 64)
		sFloat, _ := strconv.ParseFloat(strings.TrimSpace(timeParts[1]), 64)
		totalSeconds += m*60 + int64(sFloat)
		return totalSeconds
	}

	return totalSeconds
}

// getMemoryInfo retrieves memory information dynamically and robustly
func (c *Client) getMemoryInfo(ctx context.Context, device models.Device, metrics *DeviceMetrics) error {
	// Method 1: Walk hrStorageTable to find the Physical RAM index.
	// This is the most robust standard way to query memory for Windows, Linux, MikroTik, etc.
	// Walk OIDStorageType ("1.3.6.1.2.1.25.2.3.1.2") to identify RAM
	storageTypes, err := c.WalkOIDByIndex(ctx, device.IPAddress, OIDStorageType, device.SNMPCommunity)
	if err == nil && len(storageTypes) > 0 {
		unitsMap, _ := c.WalkOIDByIndex(ctx, device.IPAddress, OIDStorageAllocationUnits, device.SNMPCommunity)
		sizesMap, _ := c.WalkOIDByIndex(ctx, device.IPAddress, OIDStorageSize, device.SNMPCommunity)
		usedsMap, _ := c.WalkOIDByIndex(ctx, device.IPAddress, OIDStorageUsed, device.SNMPCommunity)
		descrMap, _ := c.WalkOIDByIndex(ctx, device.IPAddress, OIDStorageDescr, device.SNMPCommunity)

		for idx, stVal := range storageTypes {
			stStr := fmt.Sprintf("%v", stVal)
			dStr := ""
			if dVal, ok := descrMap[idx]; ok {
				dStr = strings.ToLower(c.parseString(dVal))
			}

			isRam := strings.Contains(stStr, "1.3.6.1.2.1.25.2.1.2") || stStr == "1.3.6.1.2.1.25.2.1.2" ||
				strings.Contains(dStr, "main memory") || strings.Contains(dStr, "ram") || strings.Contains(dStr, "memory")

			if isRam {
				unitsVal := unitsMap[idx]
				sizeVal := sizesMap[idx]
				usedVal := usedsMap[idx]

				units, errU2 := c.parseInt64(unitsVal)
				size, errS2 := c.parseInt64(sizeVal)
				used, errUs2 := c.parseInt64(usedVal)

				if errU2 == nil && errS2 == nil && errUs2 == nil && units > 0 && size > 0 {
					metrics.MemoryTotal = size * units
					metrics.MemoryUsed = used * units
					metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
					return nil
				}
			}
		}
	}

	// Method 2: UCD-SNMP-MIB (Linux)
	// memTotalReal = "1.3.6.1.4.1.2021.4.5.0"
	// memAvailReal = "1.3.6.1.4.1.2021.4.6.0"
	totalRealVal, errT := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.2021.4.5.0", device.SNMPCommunity)
	availRealVal, errA := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.4.1.2021.4.6.0", device.SNMPCommunity)
	if errT == nil && errA == nil {
		totalReal, errT2 := c.parseInt64(totalRealVal)
		availReal, errA2 := c.parseInt64(availRealVal)
		if errT2 == nil && errA2 == nil && totalReal > 0 {
			metrics.MemoryTotal = totalReal * 1024 // KB to Bytes
			metrics.MemoryUsed = (totalReal - availReal) * 1024
			metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
			c.logger.Debug("Retrieved memory via UCD-SNMP-MIB", map[string]interface{}{
				"device": device.Name, "total": metrics.MemoryTotal, "used": metrics.MemoryUsed,
			})
			return nil
		}
	}

	// Method 3: Cisco Memory Pool
	// ciscoMemoryPoolUsed = "1.3.6.1.4.1.9.9.48.1.1.1.5.1"
	// ciscoMemoryPoolFree = "1.3.6.1.4.1.9.9.48.1.1.1.6.1"
	ciscoUsedVal, errCU := c.QueryOID(ctx, device.IPAddress, OIDCiscoMemoryPoolUsed, device.SNMPCommunity)
	ciscoFreeVal, errCF := c.QueryOID(ctx, device.IPAddress, OIDCiscoMemoryPoolFree, device.SNMPCommunity)
	if errCU == nil && errCF == nil {
		ciscoUsed, errCU2 := c.parseInt64(ciscoUsedVal)
		ciscoFree, errCF2 := c.parseInt64(ciscoFreeVal)
		if errCU2 == nil && errCF2 == nil && (ciscoUsed+ciscoFree) > 0 {
			metrics.MemoryUsed = ciscoUsed
			metrics.MemoryTotal = ciscoUsed + ciscoFree
			metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
			c.logger.Debug("Retrieved memory via Cisco Memory Pool MIB", map[string]interface{}{
				"device": device.Name, "total": metrics.MemoryTotal, "used": metrics.MemoryUsed,
			})
			return nil
		}
	}

	// Method 4: MikroTik specific index 65536 direct fallback
	// Units: 1.3.6.1.2.1.25.2.3.1.4.65536
	// Size: 1.3.6.1.2.1.25.2.3.1.5.65536
	// Used: 1.3.6.1.2.1.25.2.3.1.6.65536
	unitsVal, errU := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.2.1.25.2.3.1.4.65536", device.SNMPCommunity)
	sizeVal, errS := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.2.1.25.2.3.1.5.65536", device.SNMPCommunity)
	usedVal, errUs := c.QueryOID(ctx, device.IPAddress, "1.3.6.1.2.1.25.2.3.1.6.65536", device.SNMPCommunity)
	if errU == nil && errS == nil && errUs == nil {
		units, errU2 := c.parseInt64(unitsVal)
		size, errS2 := c.parseInt64(sizeVal)
		used, errUs2 := c.parseInt64(usedVal)
		if errU2 == nil && errS2 == nil && errUs2 == nil && units > 0 && size > 0 {
			metrics.MemoryTotal = size * units
			metrics.MemoryUsed = used * units
			metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
			c.logger.Debug("Retrieved memory via MikroTik direct index 65536", map[string]interface{}{
				"device": device.Name, "total": metrics.MemoryTotal, "used": metrics.MemoryUsed,
			})
			return nil
		}
	}

	// Method 5: Fallback to standard hrMemorySize (Total) and hrStorageUsed.1 or hrStorageUsed.65536 using safe parsing
	// Try standard Total Memory: OIDMemorySize = "1.3.6.1.2.1.25.2.2.0"
	var totalMemory int64
	if totalVal, err := c.QueryOID(ctx, device.IPAddress, OIDMemorySize, device.SNMPCommunity); err == nil {
		if t, err2 := c.parseInt64(totalVal); err2 == nil {
			totalMemory = t * 1024 // KB to Bytes
		}
	}

	// Try standard Used Memory index 1: OIDMemoryUsed = "1.3.6.1.2.1.25.2.3.1.5.1"
	// Let's try BOTH hrStorageUsed.1, hrStorageUsed.65536 and hrStorageSize.1
	var usedMemory int64
	for _, usedOID := range []string{"1.3.6.1.2.1.25.2.3.1.6.1", "1.3.6.1.2.1.25.2.3.1.6.65536", "1.3.6.1.2.1.25.2.3.1.5.1"} {
		if usedVal, err := c.QueryOID(ctx, device.IPAddress, usedOID, device.SNMPCommunity); err == nil {
			if u, err2 := c.parseInt64(usedVal); err2 == nil && u > 0 {
				usedMemory = u * 1024
				break
			}
		}
	}

	if totalMemory > 0 && usedMemory > 0 {
		metrics.MemoryTotal = totalMemory
		metrics.MemoryUsed = usedMemory
		metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
		c.logger.Debug("Retrieved memory via fallback MIB", map[string]interface{}{
			"device": device.Name, "total": metrics.MemoryTotal, "used": metrics.MemoryUsed,
		})
		return nil
	}

	c.logger.Warn("Could not retrieve memory usage from any OID", map[string]interface{}{
		"device": device.Name,
		"ip":     device.IPAddress,
	})
	return fmt.Errorf("failed to retrieve memory info")
}

// collectInterfaceMetrics collects network interface metrics using SNMP WALK for efficiency
func (c *Client) collectInterfaceMetrics(ctx context.Context, device models.Device, metrics *DeviceMetrics) error {
	names, err := c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfName, device.SNMPCommunity)
	if err != nil || len(names) == 0 {
		names, err = c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfDescr, device.SNMPCommunity)
		if err != nil {
			return fmt.Errorf("walk ifName/ifDescr failed: %w", err)
		}
	}

	statuses, err := c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfOperStatus, device.SNMPCommunity)
	if err != nil {
		return fmt.Errorf("walk ifStatus failed: %w", err)
	}

	inOctets, err := c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfHCInOctets, device.SNMPCommunity)
	if err != nil || len(inOctets) == 0 {
		inOctets, err = c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfInOctets, device.SNMPCommunity)
		if err != nil {
			return fmt.Errorf("walk ifInOctets failed: %w", err)
		}
	}

	outOctets, err := c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfHCOutOctets, device.SNMPCommunity)
	if err != nil || len(outOctets) == 0 {
		outOctets, err = c.WalkOIDByIndex(ctx, device.IPAddress, OIDIfOutOctets, device.SNMPCommunity)
		if err != nil {
			return fmt.Errorf("walk ifOutOctets failed: %w", err)
		}
	}

	// Fetch optional OIDs concurrently with a short timeout so slow/unsupported OIDs (like ifAlias) don't stall collection
	var speeds, inErrors, outErrors, inDiscards, outDiscards, aliases, descrs map[int]interface{}
	var wg sync.WaitGroup
	optCtx, optCancel := context.WithTimeout(ctx, 10*time.Second)
	defer optCancel()

	wg.Add(6)
	go func() {
		defer wg.Done()
		if s, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfHighSpeed, device.SNMPCommunity); err == nil && len(s) > 0 {
			speeds = s
		} else if s2, err2 := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfSpeed, device.SNMPCommunity); err2 == nil {
			speeds = s2
		}
	}()
	go func() {
		defer wg.Done()
		if ie, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfInErrors, device.SNMPCommunity); err == nil {
			inErrors = ie
		}
	}()
	go func() {
		defer wg.Done()
		if oe, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfOutErrors, device.SNMPCommunity); err == nil {
			outErrors = oe
		}
	}()
	go func() {
		defer wg.Done()
		if id, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfInDiscards, device.SNMPCommunity); err == nil {
			inDiscards = id
		}
		if od, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfOutDiscards, device.SNMPCommunity); err == nil {
			outDiscards = od
		}
	}()
	go func() {
		defer wg.Done()
		if al, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfAlias, device.SNMPCommunity); err == nil {
			aliases = al
		}
	}()
	go func() {
		defer wg.Done()
		if d, err := c.WalkOIDByIndex(optCtx, device.IPAddress, OIDIfDescr, device.SNMPCommunity); err == nil {
			descrs = d
		}
	}()

	wg.Wait()

	indexes := make([]int, 0, len(names))
	for index := range names {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	for i, index := range indexes {
		if i >= 50 {
			break
		}

		name := strings.TrimSpace(c.parseString(names[index]))
		if name == "" {
			name = fmt.Sprintf("if%d", index)
		}

		aliasStr := strings.TrimSpace(c.parseString(aliases[index]))
		descrStr := strings.TrimSpace(c.parseString(descrs[index]))
		
		if aliasStr == "" && descrStr != "" && descrStr != name {
			aliasStr = descrStr
		}

		if aliasStr != "" {
			c.logger.Info("Found Alias", map[string]interface{}{"ip": device.IPAddress, "index": index, "name": name, "alias": aliasStr})
		}
		iface := InterfaceMetrics{
			DeviceID:       device.ID,
			InterfaceIndex: index,
			InterfaceName:  name,
			InterfaceAlias: aliasStr,
			Status:         "unknown",
			CollectedAt:    time.Now(),
		}

		if statusVal, err := c.parseInt64(statuses[index]); err == nil {
			switch statusVal {
			case 1:
				iface.Status = "up"
			case 2:
				iface.Status = "down"
			case 3:
				iface.Status = "testing"
			default:
				iface.Status = "unknown"
			}
		}

		if in, err := c.parseInt64(inOctets[index]); err == nil {
			iface.InOctets = in
		}
		if out, err := c.parseInt64(outOctets[index]); err == nil {
			iface.OutOctets = out
		}
		if speed, err := c.parseInt64(speeds[index]); err == nil {
			if len(speeds) > 0 && speed > 0 && speed < 1000000 {
				speed *= 1000000
			}
			iface.Speed = speed
		}
		if value, err := c.parseInt64(inErrors[index]); err == nil {
			iface.InErrors = value
		}
		if value, err := c.parseInt64(outErrors[index]); err == nil {
			iface.OutErrors = value
		}
		if value, err := c.parseInt64(inDiscards[index]); err == nil {
			iface.InDiscards = value
		}
		if value, err := c.parseInt64(outDiscards[index]); err == nil {
			iface.OutDiscards = value
		}

		metrics.Interfaces = append(metrics.Interfaces, iface)
	}

	return nil
}

func (c *Client) parseString(v interface{}) string {
	var s string
	switch val := v.(type) {
	case string:
		s = val
	case []byte:
		s = string(val)
	default:
		return ""
	}
	return strings.Trim(s, "\x00 \t\r\n")
}

func (c *Client) parseInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case uint:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint64:
		if val > uint64(^uint64(0)>>1) {
			return 0, fmt.Errorf("integer overflows int64: %d", val)
		}
		return int64(val), nil
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
			return parsed, nil
		}
	}
	return 0, fmt.Errorf("not an integer: %T", v)
}

// DetectBrand tries to identify the device manufacturer via SNMP sysDescr
func (c *Client) DetectBrand(ctx context.Context, ip string, community string) string {
	client := c.createGoSNMP(ip, community, 1, 1000) // Fast timeout for detection
	if err := client.Connect(); err != nil {
		return "Router" // Default if unreachable
	}
	defer client.Conn.Close()

	oids := []string{".1.3.6.1.2.1.1.1.0"}
	pkt, err := c.queryGet(ctx, client, oids)
	if err != nil || len(pkt.Variables) == 0 {
		return "Router"
	}

	var descrStr string
	switch v := pkt.Variables[0].Value.(type) {
	case string:
		descrStr = v
	case []byte:
		descrStr = string(v)
	default:
		descrStr = fmt.Sprintf("%v", v)
	}
	descr := strings.ToLower(descrStr)

	switch {
	case strings.Contains(descr, "mikrotik"), strings.Contains(descr, "routeros"), strings.Contains(descr, "routerboard"):
		return "MikroTik"
	case strings.Contains(descr, "ubnt"), strings.Contains(descr, "ubiquiti"), strings.Contains(descr, "unifi"):
		return "Ubiquiti"
	case strings.Contains(descr, "ruijie"), strings.Contains(descr, "reyee"):
		return "Ruijie"
	case strings.Contains(descr, "cisco"):
		return "Cisco"
	case strings.Contains(descr, "juniper"):
		return "Juniper"
	case strings.Contains(descr, "huawei"):
		return "Huawei"
	case strings.Contains(descr, "windows"):
		return "Windows"
	case strings.Contains(descr, "linux"):
		return "Linux"
	default:
		return "Router"
	}
}

func (c *Client) applyRealisticJitter(_ models.Device, _ *DeviceMetrics) {
	// Jitter is disabled to ensure 1:1 sync with physical hardware metrics.
}

// bandwidthHistory stores previous octets for delta calculation
type bandwidthHist struct {
	mu      sync.Mutex
	history map[string]*interfaceHistory
	maxAge  time.Duration
}

var bwHist = &bandwidthHist{
	history: make(map[string]*interfaceHistory),
	maxAge:  10 * time.Minute,
}

func (bh *bandwidthHist) get(key string) *interfaceHistory {
	bh.mu.Lock()
	defer bh.mu.Unlock()
	if v, ok := bh.history[key]; ok {
		return v
	}
	return nil
}

func (bh *bandwidthHist) store(key string, val *interfaceHistory) {
	bh.mu.Lock()
	defer bh.mu.Unlock()
	bh.history[key] = val
	bh.cleanup()
}

func (bh *bandwidthHist) cleanup() {
	now := time.Now()
	for k, v := range bh.history {
		if now.Sub(v.lastTime) > bh.maxAge {
			delete(bh.history, k)
		}
	}
}

// selectPrimaryInterface selects the main interface for bandwidth based on device brand
// UNUSED - func selectPrimaryInterface(brand string, interfaces []InterfaceMetrics) *InterfaceMetrics {
// 	priorityPatterns := []string{}
//
// 	switch strings.ToLower(brand) {
// 	case "mikrotik":
// 		priorityPatterns = []string{"ether1", "sfp-sfpplus1", "wan"}
// 	case "cisco":
// 		priorityPatterns = []string{"gi0/0", "gi0/1", "ten"}
// 	case "ubiquiti":
// 		priorityPatterns = []string{"eth0", "eth1", "wan"}
// 	}
//
// 	// Find interface matching priority patterns
// 	for _, pattern := range priorityPatterns {
// 		for i := range interfaces {
// 			if strings.EqualFold(interfaces[i].InterfaceName, pattern) ||
// 				strings.Contains(strings.ToLower(interfaces[i].InterfaceName), pattern) {
// 				return &interfaces[i]
// 			}
// 		}
// 	}
//
// 	// If no priority match, find interface with highest traffic
// 	var maxIface *InterfaceMetrics
// 	maxTraffic := int64(0)
// 	for i := range interfaces {
// 		if interfaces[i].Status == "up" {
// 			traffic := interfaces[i].InOctets + interfaces[i].OutOctets
// 			if traffic > maxTraffic {
// 				maxTraffic = traffic
// 				maxIface = &interfaces[i]
// 			}
// 		}
// 	}
//
// 	if maxIface != nil {
// 		return maxIface
// 	}
//
// 	// Fallback to first up interface
// 	for i := range interfaces {
// 		if interfaces[i].Status == "up" {
// 			return &interfaces[i]
// 		}
// 	}
//
// 	return nil
// }

// collectWirelessMetrics fetches metrics specific to Radios like Signal, CCQ, Tx/Rx rates.
// brand parameter is passed from CollectDeviceMetrics (resolved via getEffectiveBrand) to avoid
// re-detecting brand from sysDescr which is unreliable for Ubiquiti devices.
func (c *Client) collectWirelessMetrics(ctx context.Context, device models.Device, metrics *DeviceMetrics, brand string) {
	c.logger.Info("Collect wireless metrics", map[string]interface{}{
		"device": device.Name,
		"ip":     device.IPAddress,
		"brand":  brand,
	})

	if strings.EqualFold(brand, "MikroTik") {

		// ==========================
		// MikroTik Wireless
		// ==========================

		// SIGNAL (mtxrWlStatRssi: 1.3.6.1.4.1.14988.1.1.1.2.1.3)
		if strengths, err := c.WalkOID(
			ctx,
			device.IPAddress,
			"1.3.6.1.4.1.14988.1.1.1.2.1.3",
			device.SNMPCommunity,
		); err == nil && len(strengths) > 0 {

			if val, err := c.parseInt64(strengths[0]); err == nil {
				metrics.SignalStrength = float64(val)
			}
		}

		// CCQ (mtxrWlStatTxCCQ: 1.3.6.1.4.1.14988.1.1.1.3.1.7 or mtxrWlStatOverallTxCCQ: ...1.3.1.10)
		if ccqs, err := c.WalkOID(
			ctx,
			device.IPAddress,
			"1.3.6.1.4.1.14988.1.1.1.3.1.7",
			device.SNMPCommunity,
		); err == nil && len(ccqs) > 0 {

			if val, err := c.parseInt64(ccqs[0]); err == nil {
				ccq := float64(val)
				if ccq > 100 {
					ccq /= 100.0 // MikroTik often returns 5500 for 55%
				}
				metrics.CCQ = ccq
			}
		}

		// TX RATE
		if txRates, err := c.WalkOID(
			ctx,
			device.IPAddress,
			"1.3.6.1.4.1.14988.1.1.1.2.1.8",
			device.SNMPCommunity,
		); err == nil && len(txRates) > 0 {

			if val, err := c.parseInt64(txRates[0]); err == nil {
				metrics.TxRate = float64(val) / 1000000.0
			}
		}

		// RX RATE
		if rxRates, err := c.WalkOID(
			ctx,
			device.IPAddress,
			"1.3.6.1.4.1.14988.1.1.1.2.1.9",
			device.SNMPCommunity,
		); err == nil && len(rxRates) > 0 {

			if val, err := c.parseInt64(rxRates[0]); err == nil {
				metrics.RxRate = float64(val) / 1000000.0
			}
		}

	} else if strings.EqualFold(brand, "Ubiquiti") {

		// ==========================
		// Ubiquiti Wireless (AirOS M / AirOS AC / AirMax)
		// OID Reference: UBNT-AirMAX-MIB (1.3.6.1.4.1.41112.1.4)
		// Always use WalkOID â€” index .1 is not guaranteed to exist
		// ==========================

		// SSID (ubntWlStatSsid: 1.3.6.1.4.1.41112.1.4.5.1.2)
		if ssidVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.2", device.SNMPCommunity); err == nil && len(ssidVals) > 0 {
			metrics.SSID = c.parseString(ssidVals[0])
		}

		// Frequency (ubntAirFiber24GHz / ubntRadioFreq: 1.3.6.1.4.1.41112.1.4.1.1.4)
		if freqVals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.1.1.4", device.SNMPCommunity); err == nil && len(freqVals) > 0 {
			metrics.Frequency = c.parseString(freqVals[0])
		}

		// ---- SIGNAL STRENGTH ----
		// Try 1: ubntWlStatRssi (Rx Signal from remote): 1.3.6.1.4.1.41112.1.4.5.1.5 â€” most reliable for M-series
		signalOK := false
		vals, err1 := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.5", device.SNMPCommunity)
		if err1 == nil && len(vals) > 0 {
			if v, err := c.parseInt64(vals[0]); err == nil && v != 0 {
				metrics.SignalStrength = float64(v)
				signalOK = true
			}
		} else {
			c.logger.Debug("Ubiquiti Try 1 Signal Failed", map[string]interface{}{"err": err1, "vals": len(vals), "ip": device.IPAddress})
		}
		// Try 2: ubntAirMaxStaRssi for AC firmware: 1.3.6.1.4.1.41112.1.4.7.1.3
		if !signalOK {
			vals2, err2 := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.7.1.3", device.SNMPCommunity)
			if err2 == nil && len(vals2) > 0 {
				if v, err := c.parseInt64(vals2[0]); err == nil && v != 0 {
					metrics.SignalStrength = float64(v)
					signalOK = true
				}
			} else {
				c.logger.Debug("Ubiquiti Try 2 Signal Failed", map[string]interface{}{"err": err2, "vals": len(vals2), "ip": device.IPAddress})
			}
		}
		// Try 3: Alternative signal OID for older AirOS: 1.3.6.1.4.1.41112.1.4.5.1.4
		if !signalOK {
			if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.4", device.SNMPCommunity); err == nil && len(vals) > 0 {
				if v, err := c.parseInt64(vals[0]); err == nil && v != 0 {
					metrics.SignalStrength = float64(v)
				}
			}
		}

		// ---- CCQ ----
		// Try 1: ubntWlStatCcq (M-series): 1.3.6.1.4.1.41112.1.4.5.1.7
		ccqOK := false
		if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.7", device.SNMPCommunity); err == nil && len(vals) > 0 {
			if v, err := c.parseInt64(vals[0]); err == nil {
				ccq := float64(v)
				if ccq > 100 {
					ccq /= 10.0 // Some firmware returns 0-1000
				}
				metrics.CCQ = ccq
				ccqOK = true
			}
		}
		// Try 2: AirMax Quality (AC firmware): 1.3.6.1.4.1.41112.1.4.6.1.2
		if !ccqOK {
			if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.6.1.2", device.SNMPCommunity); err == nil && len(vals) > 0 {
				if v, err := c.parseInt64(vals[0]); err == nil {
					metrics.CCQ = float64(v)
					ccqOK = true
				}
			}
		}
		// Try 3: ubntAirMaxStaCcq (AC station): 1.3.6.1.4.1.41112.1.4.7.1.10
		if !ccqOK {
			if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.7.1.10", device.SNMPCommunity); err == nil && len(vals) > 0 {
				if v, err := c.parseInt64(vals[0]); err == nil {
					metrics.CCQ = float64(v)
				}
			}
		}

		// ---- TX RATE ----
		// ubntWlStatTxRate: 1.3.6.1.4.1.41112.1.4.5.1.9 â€” value is in bps or Kbps on M-series
		txOK := false
		if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.9", device.SNMPCommunity); err == nil && len(vals) > 0 {
			if v, err := c.parseInt64(vals[0]); err == nil && v > 0 {
				if v >= 1000000 {
					metrics.TxRate = float64(v) / 1000000.0 // bps to Mbps
				} else if v >= 1000 {
					metrics.TxRate = float64(v) / 1000.0 // Kbps to Mbps
				} else {
					metrics.TxRate = float64(v)
				}
				txOK = true
			}
		}
		// Fallback: AC firmware ubntAirMaxStaTxRate: 1.3.6.1.4.1.41112.1.4.7.1.5
		if !txOK {
			if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.7.1.5", device.SNMPCommunity); err == nil && len(vals) > 0 {
				if v, err := c.parseInt64(vals[0]); err == nil && v > 0 {
					if v >= 1000000 {
						metrics.TxRate = float64(v) / 1000000.0
					} else if v >= 1000 {
						metrics.TxRate = float64(v) / 1000.0
					} else {
						metrics.TxRate = float64(v)
					}
				}
			}
		}

		// ---- RX RATE ----
		// ubntWlStatRxRate: 1.3.6.1.4.1.41112.1.4.5.1.10 â€” value is in bps or Kbps on M-series
		rxOK := false
		if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.5.1.10", device.SNMPCommunity); err == nil && len(vals) > 0 {
			if v, err := c.parseInt64(vals[0]); err == nil && v > 0 {
				if v >= 1000000 {
					metrics.RxRate = float64(v) / 1000000.0 // bps to Mbps
				} else if v >= 1000 {
					metrics.RxRate = float64(v) / 1000.0 // Kbps to Mbps
				} else {
					metrics.RxRate = float64(v)
				}
				rxOK = true
			}
		}
		// Fallback: AC firmware ubntAirMaxStaRxRate: 1.3.6.1.4.1.41112.1.4.7.1.6
		if !rxOK {
			if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.7.1.6", device.SNMPCommunity); err == nil && len(vals) > 0 {
				if v, err := c.parseInt64(vals[0]); err == nil && v > 0 {
					if v >= 1000000 {
						metrics.RxRate = float64(v) / 1000000.0
					} else if v >= 1000 {
						metrics.RxRate = float64(v) / 1000.0
					} else {
						metrics.RxRate = float64(v)
					}
				}
			}
		}

		// Airtime (ubntAirMaxCapacity: 1.3.6.1.4.1.41112.1.4.6.1.7)
		if vals, err := c.WalkOID(ctx, device.IPAddress, "1.3.6.1.4.1.41112.1.4.6.1.7", device.SNMPCommunity); err == nil && len(vals) > 0 {
			if v, err := c.parseInt64(vals[0]); err == nil {
				metrics.Airtime = float64(v)
			}
		}

		c.logger.Debug("Ubiquiti wireless metrics collected", map[string]interface{}{
			"device": device.Name,
			"signal": metrics.SignalStrength,
			"ccq":    metrics.CCQ,
			"txRate": metrics.TxRate,
			"rxRate": metrics.RxRate,
		})

	} else {
		// Generic / Standard IEEE 802.11 Wireless MIB Fallbacks (TP-Link, Ruijie, etc.)
		if ssidVals, err := c.WalkOID(ctx, device.IPAddress, "1.2.840.10036.1.1.1.9", device.SNMPCommunity); err == nil && len(ssidVals) > 0 {
			metrics.SSID = c.parseString(ssidVals[0])
		}
		if sigVals, err := c.WalkOID(ctx, device.IPAddress, "1.2.840.10036.3.1.2.1.4", device.SNMPCommunity); err == nil && len(sigVals) > 0 {
			if v, err := c.parseInt64(sigVals[0]); err == nil && v != 0 {
				metrics.SignalStrength = float64(v)
			}
		}
		if txVals, err := c.WalkOID(ctx, device.IPAddress, "1.2.840.10036.2.2.1.3", device.SNMPCommunity); err == nil && len(txVals) > 0 {
			if v, err := c.parseInt64(txVals[0]); err == nil && v > 0 {
				metrics.TxRate = float64(v) / 1000000.0
			}
		}
		if rxVals, err := c.WalkOID(ctx, device.IPAddress, "1.2.840.10036.2.2.1.4", device.SNMPCommunity); err == nil && len(rxVals) > 0 {
			if v, err := c.parseInt64(rxVals[0]); err == nil && v > 0 {
				metrics.RxRate = float64(v) / 1000000.0
			}
		}
	}

	c.logger.Info("Wireless metrics collected", map[string]interface{}{
		"device": device.Name,
		"signal": metrics.SignalStrength,
		"ccq":    metrics.CCQ,
		"tx":     metrics.TxRate,
		"rx":     metrics.RxRate,
	})
}
