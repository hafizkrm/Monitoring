package snmp

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/gosnmp/gosnmp"
	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/logger"
)

type Client struct {
	config  config.SNMPConfig
	fullCfg *config.Config
	logger  logger.Logger

	sessionCache       sync.Map
	deviceVersionCache sync.Map
	metricCache        MetricCacheStore
	ticker             *time.Ticker
}

type cachedSession struct {
	client   *gosnmp.GoSNMP
	lastUsed time.Time
}

func NewClient(fullCfg *config.Config, log logger.Logger) *Client {
	c := &Client{
		config:  fullCfg.SNMP,
		fullCfg: fullCfg,
		logger:  log,
	}

	c.ticker = time.NewTicker(60 * time.Second)
	go c.cleanupSessions()

	return c
}

func (c *Client) cleanupSessions() {
	for range c.ticker.C {
		now := time.Now()

		c.sessionCache.Range(func(key, value interface{}) bool {
			s := value.(*cachedSession)

			if now.Sub(s.lastUsed) > 2*time.Minute {
				if s.client.Conn != nil {
					s.client.Conn.Close()
				}

				c.sessionCache.Delete(key)
			}

			return true
		})
	}
}

// cacheKey generates a unique key for the session cache
func (c *Client) cacheKey(ip, community string) string {
	// Use a format that minimizes collision risk
	return fmt.Sprintf("%s:%s", ip, community)
}

func (c *Client) getSession(ip, community string) (*gosnmp.GoSNMP, error) {
	if community == "" {
		if c.config.Community != "" {
			community = c.config.Community
		} else {
			community = "public"
		}
	}

	key := c.cacheKey(ip, community)

	version := c.parseVersion(c.config.Version)
	if cachedVer, ok := c.deviceVersionCache.Load(ip); ok {
		version = cachedVer.(gosnmp.SnmpVersion)
	}

	if val, ok := c.sessionCache.Load(key); ok {
		s := val.(*cachedSession)
		s.lastUsed = time.Now()
		s.client.Version = version
		return s.client, nil
	}

	client := &gosnmp.GoSNMP{
		Target:             ip,
		Port:               uint16(c.config.Port),
		Community:          community,
		Version:            version,
		Timeout:            c.config.GetTimeout(),
		Retries:            3,
		MaxOids:            15,
		MaxRepetitions:     10,
		ExponentialTimeout: false,
	}

	if err := client.Connect(); err != nil {
		return nil, err
	}

	c.sessionCache.Store(key, &cachedSession{
		client:   client,
		lastUsed: time.Now(),
	})

	return client, nil
}

func (c *Client) QueryOID(
	ctx context.Context,
	ip string,
	oid string,
	community string,
) (interface{}, error) {

	result, err := c.QueryMultipleOIDs(
		ctx,
		ip,
		[]string{oid},
		community,
	)

	if err != nil {
		return nil, err
	}

	return result[oid], nil
}

func (c *Client) QueryMultipleOIDs(
	ctx context.Context,
	ip string,
	oids []string,
	community string,
) (map[string]interface{}, error) {

	client, err := c.getSession(ip, community)

	if err != nil {
		return nil, err
	}

	pkt, err := c.queryGet(ctx, client, oids)
	if err != nil {
		return nil, err
	}

	results := map[string]interface{}{}

	for i, variable := range pkt.Variables {
		if i < len(oids) {
			results[oids[i]] = variable.Value
		}
	}

	return results, nil
}

func (c *Client) WalkOID(
	ctx context.Context,
	ip string,
	oid string,
	community string,
) ([]interface{}, error) {

	client, err := c.getSession(ip, community)

	if err != nil {
		return nil, err
	}

	results := make([]interface{}, 0, 64)

	err = c.queryBulkWalk(ctx, client, oid, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu.Value)
		return nil
	})

	return results, err
}

func (c *Client) WalkOIDByIndex(
	ctx context.Context,
	ip string,
	oid string,
	community string,
) (map[int]interface{}, error) {

	client, err := c.getSession(ip, community)

	if err != nil {
		return nil, err
	}

	results := make(map[int]interface{}, 64)

	err = c.queryBulkWalk(ctx, client, oid, func(pdu gosnmp.SnmpPDU) error {
		index, err := oidIndex(pdu.Name)
		if err != nil {
			return nil
		}
		results[index] = pdu.Value
		return nil
	})

	return results, err
}

func oidIndex(name string) (int, error) {
	parts := strings.Split(strings.TrimPrefix(name, "."), ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("missing oid index")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func (c *Client) parseVersion(version string) gosnmp.SnmpVersion {

	switch version {

	case "1":
		return gosnmp.Version1

	case "3":
		return gosnmp.Version3

	default:
		return gosnmp.Version2c
	}
}

func (c *Client) createGoSNMP(
	target,
	community string,
	retries int,
	timeoutMs int,
) *gosnmp.GoSNMP {

	return &gosnmp.GoSNMP{
		Target:    target,
		Port:      uint16(c.config.Port),
		Community: community,
		Version:   c.parseVersion(c.config.Version),
		Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		Retries:   retries,
	}
}

func (c *Client) GetNetworkStats(ctx context.Context, ip string) (latency int, jitter float64, loss float64, err error) {
	// On Windows, native go-ping often silently fails without Admin privileges and blocks for 2 seconds.
	// We use the OS ping directly to ensure reliability and speed.
	if runtime.GOOS == "windows" {
		if lat, ok := fallbackOSPing(ctx, ip); ok {
			return lat, 0.5, 0.0, nil
		}
		return 0, 0, 100, fmt.Errorf("no reply from %s (OS Ping)", ip)
	}

	pinger, err := ping.NewPinger(ip)
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

	// Fallback to system OS ping (guaranteed to work on Windows without Administrator privileges)
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

	// Check negative error patterns first (unreachable / timeout)
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

	// Match positive ICMP response in both English & Indonesian OS
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

// getRateCalculationInterval returns the rate calculation interval as time.Duration
// UNUSED - func (c *Client) getRateCalculationInterval() time.Duration {
// 	if c.fullCfg == nil || c.fullCfg.Polling.RateCalculationInterval == "" {
// 		return 30 * time.Second // Default 30 seconds if not set
// 	}
// 	d, err := time.ParseDuration(c.fullCfg.Polling.RateCalculationInterval)
// 	if err != nil || d <= 0 {
// 		return 30 * time.Second
// 	}
// 	return d
// }

// queryGet performs a GET operation synchronously to avoid goroutine retention
func (c *Client) queryGet(ctx context.Context, client *gosnmp.GoSNMP, oids []string) (*gosnmp.SnmpPacket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pkt, err := client.Get(oids)
	if err != nil && client.Version == gosnmp.Version2c {
		client.Version = gosnmp.Version1
		pkt, err = client.Get(oids)
		if err == nil {
			c.deviceVersionCache.Store(client.Target, gosnmp.Version1)
		}
	} else if err == nil && client.Version == gosnmp.Version1 {
		c.deviceVersionCache.Store(client.Target, gosnmp.Version1)
	}

	if err != nil {
		return nil, err
	}
	if pkt == nil {
		return nil, fmt.Errorf("empty SNMP response from %s", client.Target)
	}
	return pkt, nil
}

// queryBulkWalk performs a BulkWalk operation synchronously to avoid goroutine retention
func (c *Client) queryBulkWalk(ctx context.Context, client *gosnmp.GoSNMP, oid string, fn func(gosnmp.SnmpPDU) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var err error
	if client.Version == gosnmp.Version1 {
		err = client.Walk(oid, fn)
		if err == nil {
			c.deviceVersionCache.Store(client.Target, gosnmp.Version1)
		}
	} else {
		err = client.BulkWalk(oid, fn)
		if err != nil {
			client.Version = gosnmp.Version1
			err = client.Walk(oid, fn)
			if err == nil {
				c.deviceVersionCache.Store(client.Target, gosnmp.Version1)
			}
		}
	}
	
	// Check context again after walk finishes
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// Close stops the session cleanup ticker and releases resources
func (c *Client) Close() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
}
