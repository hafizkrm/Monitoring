package snmp

import (
	"fmt"
	"strconv"
	"strings"
)

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
