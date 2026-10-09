package websocket

import "testing"

func TestViewerWebSocketAllowlist(t *testing.T) {
	viewer := &Client{role: "viewer"}
	for _, topic := range []string{"global", "device:42"} {
		if !viewer.CanSubscribe(topic) {
			t.Errorf("viewer should subscribe to %q", topic)
		}
	}
	for _, topic := range []string{"all", "users", "logs"} {
		if viewer.CanSubscribe(topic) {
			t.Errorf("viewer must not subscribe to %q", topic)
		}
	}
	for _, event := range []string{"metrics.updated", "alert.created", "device.connected"} {
		if !viewer.CanReceive(event) {
			t.Errorf("viewer should receive %q", event)
		}
	}
	if viewer.CanReceive("log.created") {
		t.Fatal("viewer must not receive log events")
	}
}

func TestAdminCanUseWebSocketTopicsAndEvents(t *testing.T) {
	admin := &Client{role: "admin"}
	if !admin.CanSubscribe("all") || !admin.CanReceive("log.created") {
		t.Fatal("admin should retain unrestricted event access")
	}
}
