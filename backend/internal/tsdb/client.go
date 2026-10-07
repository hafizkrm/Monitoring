package tsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type PrometheusClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewPrometheusClient(baseURL string) *PrometheusClient {
	if baseURL == "" {
		baseURL = "http://localhost:9090"
	}
	return &PrometheusClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type QueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// FetchGlobalBandwidthHistory queries Prometheus for the aggregated RX and TX rates
// of all gateway/router/firewall devices over the last 2 hours.
func (c *PrometheusClient) FetchGlobalBandwidthHistory(ctx context.Context, durationStr string) ([]map[string]interface{}, error) {
	end := time.Now()

	// Default to 30m if invalid or empty
	dur, err := time.ParseDuration(durationStr)
	if err != nil || dur <= 0 {
		dur = 30 * time.Minute
	}
	start := end.Add(-dur)

	// Dynamically calculate step based on duration to avoid too many points
	step := "10s"
	if dur > 12*time.Hour {
		step = "5m"
	} else if dur > 2*time.Hour {
		step = "1m"
	} else if dur > 1*time.Hour {
		step = "30s"
	}

	// Query for RX
	rxQuery := `sum(nms_device_rx_rate_mbps{device_type=~".*(router|firewall|gateway).*"})`
	rxData, err := c.queryRange(ctx, rxQuery, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query RX: %w", err)
	}

	// Query for TX
	txQuery := `sum(nms_device_tx_rate_mbps{device_type=~".*(router|firewall|gateway).*"})`
	txData, err := c.queryRange(ctx, txQuery, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query TX: %w", err)
	}

	// Combine results
	// They should have the same timestamps if Prometheus returns them aligned.
	historyMap := make(map[string]map[string]interface{})
	var timestamps []string

	processResult := func(res *QueryRangeResponse, field string) {
		if len(res.Data.Result) == 0 {
			return
		}
		for _, val := range res.Data.Result[0].Values {
			if len(val) == 2 {
				// val[0] is unix timestamp float64, val[1] is value string
				tFloat, ok := val[0].(float64)
				if !ok {
					continue
				}
				tStr := val[1].(string)
				var vFloat float64
				fmt.Sscanf(tStr, "%f", &vFloat)

				ts := time.Unix(int64(tFloat), 0)
				timeLabel := ts.Format("15:04:05")

				if _, exists := historyMap[timeLabel]; !exists {
					historyMap[timeLabel] = map[string]interface{}{
						"timestamp": timeLabel,
						"download":  0.0,
						"upload":    0.0,
					}
					timestamps = append(timestamps, timeLabel)
				}
				historyMap[timeLabel][field] = vFloat
			}
		}
	}

	processResult(rxData, "download")
	processResult(txData, "upload")

	var results []map[string]interface{}
	for _, t := range timestamps {
		results = append(results, historyMap[t])
	}

	// Return top 30 like DB
	if len(results) > 30 {
		results = results[len(results)-30:]
	}

	return results, nil
}

// FetchDeviceMetricsHistory returns CPU, memory, latency, packet_loss history for a single device ID.
func (c *PrometheusClient) FetchDeviceMetricsHistory(ctx context.Context, deviceID string, duration time.Duration) ([]map[string]interface{}, error) {
	end := time.Now()
	start := end.Add(-duration)
	step := "30s"
	if duration > 1*time.Hour {
		step = "60s"
	}
	if duration > 6*time.Hour {
		step = "5m"
	}

	type metricQuery struct {
		query string
		field string
	}

	queries := []metricQuery{
		// Removed: Per-device telemetry gauges have been deprecated due to cardinality limits.
		// Telemetry is now strictly handled by SQL authoritative DB.
	}

	// Use a unified time-keyed map
	historyMap := make(map[int64]map[string]interface{})
	var orderedTimestamps []int64

	for _, mq := range queries {
		res, err := c.queryRange(ctx, mq.query, start, end, step)
		if err != nil {
			continue // skip individual metric errors
		}
		if len(res.Data.Result) == 0 {
			continue
		}
		for _, val := range res.Data.Result[0].Values {
			if len(val) != 2 {
				continue
			}
			tFloat, ok := val[0].(float64)
			if !ok {
				continue
			}
			tUnix := int64(tFloat)
			tStr, _ := val[1].(string)
			var vFloat float64
			fmt.Sscanf(tStr, "%f", &vFloat)

			if _, exists := historyMap[tUnix]; !exists {
				historyMap[tUnix] = map[string]interface{}{
					"timestamp":   time.Unix(tUnix, 0).Format("15:04:05"),
					"unix":        tUnix,
					"cpu":         0.0,
					"memory":      0.0,
					"latency":     0.0,
					"packet_loss": 0.0,
					"rx_rate":     0.0,
					"tx_rate":     0.0,
				}
				orderedTimestamps = append(orderedTimestamps, tUnix)
			}
			historyMap[tUnix][mq.field] = vFloat
		}
	}

	// Sort timestamps ascending
	sort.Slice(orderedTimestamps, func(i, j int) bool {
		return orderedTimestamps[i] < orderedTimestamps[j]
	})

	var results []map[string]interface{}
	for _, ts := range orderedTimestamps {
		results = append(results, historyMap[ts])
	}
	return results, nil
}

// QueryInstant runs an instant PromQL query and returns raw Prometheus response.
func (c *PrometheusClient) QueryInstant(ctx context.Context, query string) (*QueryRangeResponse, error) {
	u, _ := url.Parse(c.BaseURL + "/api/v1/query")
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status: %s", resp.Status)
	}

	var result QueryRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *PrometheusClient) queryRange(ctx context.Context, query string, start, end time.Time, step string) (*QueryRangeResponse, error) {
	u, _ := url.Parse(c.BaseURL + "/api/v1/query_range")
	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", end.Unix()))
	q.Set("step", step)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status: %s", resp.Status)
	}

	var result QueryRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
