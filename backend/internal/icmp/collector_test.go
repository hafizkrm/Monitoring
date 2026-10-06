package icmp

import (
	"context"
	"testing"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

func TestICMPCollector_Collect(t *testing.T) {
	c := NewICMPCollector()

	// Use localhost to guarantee a ping reply
	device := models.Device{
		ID:        1,
		IPAddress: "127.0.0.1",
		Hostname:  "localhost",
	}

	snapshot, err := c.Collect(context.Background(), device)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if snapshot.Status != "up" {
		t.Errorf("expected status 'up', got '%s'", snapshot.Status)
	}
	if snapshot.ReachabilityStatus != "up" {
		t.Errorf("expected reachability 'up', got '%s'", snapshot.ReachabilityStatus)
	}
	if snapshot.CPUUsage != 0 || snapshot.MemoryUsage != 0 {
		t.Errorf("expected CPU and Memory to be 0 for ICMP collector")
	}
}

func TestICMPCollector_Collect_Fail(t *testing.T) {
	c := NewICMPCollector()

	// Use a non-routable IP to force a failure
	device := models.Device{
		ID:        2,
		IPAddress: "192.0.2.1",
		Hostname:  "blackhole",
	}

	// This may return an error or return a down snapshot depending on OS behavior.
	// Either way is acceptable as long as it handles it gracefully.
	snapshot, err := c.Collect(context.Background(), device)
	if err != nil {
		// Valid outcome
		return
	}

	if snapshot.Status != "down" {
		t.Errorf("expected status 'down', got '%s'", snapshot.Status)
	}
}
