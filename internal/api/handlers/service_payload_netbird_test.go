package handlers

import (
	"testing"

	"github.com/autobrr/dashbrr/internal/services/netbird"
)

func TestNetBirdPeersServiceUpdate(t *testing.T) {
	peers := []netbird.Peer{
		{ID: "peer-1", Name: "host-1", Online: true, Groups: []string{"homelab"}, UpdateAvailable: true},
		{ID: "peer-2", Name: "host-2", Online: false, Groups: []string{"office"}},
	}

	update := buildNetBirdPeersServiceUpdate("netbird-1", peers, true)
	if update.Message != "netbird_peers" {
		t.Fatalf("message = %q, want netbird_peers", update.Message)
	}
	if update.ServiceID != "netbird-1" || update.Status != "online" {
		t.Fatalf("update = %#v", update)
	}
	if !update.UpdateAvailable {
		t.Fatalf("updateAvailable = false, want true for an outdated management server")
	}

	details, ok := update.Details["netbird"].(map[string]any)
	if !ok {
		t.Fatalf("details.netbird missing: %#v", update.Details)
	}
	if details["total"] != 2 || details["online"] != 1 {
		t.Fatalf("details = %#v, want total 2 and online 1", details)
	}

	stats, ok := update.Stats["netbird"].(map[string]any)
	if !ok {
		t.Fatalf("stats.netbird missing: %#v", update.Stats)
	}
	gotPeers, ok := stats["peers"].([]netbird.Peer)
	if !ok || len(gotPeers) != 2 {
		t.Fatalf("stats.netbird.peers = %#v", stats["peers"])
	}
	if gotPeers[0].Name != "host-1" || !gotPeers[0].UpdateAvailable {
		t.Fatalf("first peer = %#v", gotPeers[0])
	}
}

func TestNetBirdPeersServiceUpdateWithoutPeers(t *testing.T) {
	update := buildNetBirdPeersServiceUpdate("netbird-1", []netbird.Peer{}, false)

	details := update.Details["netbird"].(map[string]any)
	if details["total"] != 0 || details["online"] != 0 {
		t.Fatalf("details = %#v, want zero counts", details)
	}
	if update.UpdateAvailable {
		t.Fatalf("updateAvailable = true, want false when the version call gave nothing")
	}
}
