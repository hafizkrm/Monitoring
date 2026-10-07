package icmp

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
	probing "github.com/prometheus-community/pro-bing"
)

// ICMPCollector provides telemetry via ICMP Ping.
type ICMPCollector struct{}

// NewICMPCollector creates a new ICMPCollector
func NewICMPCollector() *ICMPCollector {
	return &ICMPCollector{}
}

// Collect returns minimal telemetry for an ICMP-only device
func (c *ICMPCollector) Collect(ctx context.Context, device models.Device) (*models.TelemetrySnapshot, error) {
	latency, jitter, loss, err := c.GetNetworkStats(ctx, device.IPAddress)
	if err != nil {
		return nil, err
	}

	status := "up"
	reachability := "up"
	if loss == 100 {
		status = "down"
		reachability = "down"
	}

	return &models.TelemetrySnapshot{
		DeviceID:           device.ID,
		Hostname:           device.Hostname,
		Uptime:             0, // Not available via ICMP
		CPUUsage:           0,
		MemoryUsage:        0,
		MemoryTotal:        0,
		MemoryUsed:         0,
		Status:             status,
		ReachabilityStatus: reachability,
		SNMPStatus:         "unknown", // Not applicable for ICMP
		CollectedAt:        time.Now(),
		LatencyMs:          latency,
		Jitter:             jitter,
		PacketLoss:         loss,
	}, nil
}

// GetNetworkStats duplicates the ping logic from SNMP to ensure 100% behavior match
func (c *ICMPCollector) GetNetworkStats(ctx context.Context, ip string) (latency int, jitter float64, loss float64, err error) {
	if runtime.GOOS == "windows" {
		if lat, ok := fallbackOSPing(ctx, ip); ok {
			return lat, 0.5, 0.0, nil
		}
		return 0, 0, 100, fmt.Errorf("no reply from %s (OS Ping)", ip)
	}

	pinger, err := probing.NewPinger(ip)
	if err == nil {
		pinger.Count = 3
		pinger.Timeout = 2 * time.Second

		errCh := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("panic in pinger: %v", r)
				}
			}()
			errCh <- pinger.Run()
		}()

		var runErr error
		select {
		case <-ctx.Done():
			pinger.Stop()
			return 0, 0, 0, ctx.Err()
		case runErr = <-errCh:
		}

		if runErr == nil {
			stats := pinger.Statistics()
			if stats.PacketsRecv > 0 {
				lat := int(stats.AvgRtt.Milliseconds())
				if lat <= 0 {
					lat = 1
				}
				return lat, float64(stats.StdDevRtt.Microseconds()) / 1000.0, stats.PacketLoss, nil
			}
		}
	}

	if lat, ok := fallbackOSPing(ctx, ip); ok {
		return lat, 0.5, 0.0, nil
	}

	return 0, 0, 100, fmt.Errorf("no reply from %s", ip)
}

func fallbackOSPing(ctx context.Context, ip string) (int, bool) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1500", ip)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", ip)
	}

	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return 0, false
	}
	outStr := strings.ToLower(string(out))

	isFailure := (strings.Contains(outStr, "unreachable") ||
		strings.Contains(outStr, "tidak dapat dijangkau") ||
		strings.Contains(outStr, "timed out") ||
		strings.Contains(outStr, "waktu permintaan habis") ||
		strings.Contains(outStr, "100% packet loss") ||
		strings.Contains(outStr, "100% kehilangan") ||
		strings.Contains(outStr, "destination host unreachable")) &&
		!strings.Contains(outStr, "ttl=") &&
		!strings.Contains(outStr, "bytes=") &&
		!strings.Contains(outStr, "byte=")

	if isFailure {
		return 0, false
	}

	hasReply := strings.Contains(outStr, "reply from") ||
		strings.Contains(outStr, "balasan dari") ||
		strings.Contains(outStr, "bytes from") ||
		strings.Contains(outStr, "bytes=") ||
		strings.Contains(outStr, "byte=") ||
		strings.Contains(outStr, "ttl=")

	if hasReply {
		lat := 10
		idx := strings.Index(outStr, "time=")
		if idx == -1 {
			idx = strings.Index(outStr, "waktu=")
		}

		if idx != -1 {
			prefixLen := 5
			if strings.HasPrefix(outStr[idx:], "waktu=") {
				prefixLen = 6
			}
			sub := outStr[idx+prefixLen:]
			if msIdx := strings.Index(sub, "ms"); msIdx > 0 {
				if parsedLat, pErr := strconv.Atoi(strings.TrimSpace(sub[:msIdx])); pErr == nil && parsedLat > 0 {
					lat = parsedLat
				}
			}
		} else if strings.Contains(outStr, "time<") || strings.Contains(outStr, "waktu<") {
			lat = 1
		}
		return lat, true
	}

	return 0, false
}
