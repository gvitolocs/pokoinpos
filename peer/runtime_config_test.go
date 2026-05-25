package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMineAttemptsForNetworkScalesWithPendingTransactions(t *testing.T) {
	cfg := NodeConfig{
		BaseMineAttemptsPerTick:  1,
		MaxMineAttemptsPerTick:   5,
		MineAttemptsPerPendingTx: 1,
	}

	if got := mineAttemptsForNetwork(0, 1, cfg); got != 1 {
		t.Fatalf("idle attempts: got %d want 1", got)
	}
	if got := mineAttemptsForNetwork(2, 1, cfg); got != 3 {
		t.Fatalf("pending attempts: got %d want 3", got)
	}
	if got := mineAttemptsForNetwork(10, 1, cfg); got != 5 {
		t.Fatalf("capped attempts: got %d want 5", got)
	}
}

func TestMineAttemptsForNetworkDividesByAvailableNodes(t *testing.T) {
	cfg := NodeConfig{
		BaseMineAttemptsPerTick:  4,
		MaxMineAttemptsPerTick:   5,
		MineAttemptsPerPendingTx: 1,
	}

	if got := mineAttemptsForNetwork(1, 1, cfg); got != 5 {
		t.Fatalf("single-node attempts: got %d want 5", got)
	}
	if got := mineAttemptsForNetwork(1, 4, cfg); got != 4 {
		t.Fatalf("four-node attempts: got %d want 4", got)
	}
	if got := mineAttemptsForNetwork(1, 0, cfg); got != 5 {
		t.Fatalf("zero availability should fall back to one node: got %d want 5", got)
	}
}

func TestShouldMineSlotBacksOffWhenIdle(t *testing.T) {
	cfg := NodeConfig{IdleSlotInterval: 300}

	if !shouldMineSlot(1, 0, cfg) {
		t.Fatal("first idle slot should mine a keepalive block")
	}
	if shouldMineSlot(2, 0, cfg) {
		t.Fatal("idle slot before interval should not mine")
	}
	if !shouldMineSlot(300, 0, cfg) {
		t.Fatal("idle slot at interval should mine")
	}
	if !shouldMineSlot(2, 1, cfg) {
		t.Fatal("pending transactions should mine immediately")
	}
}

func TestShouldMineSlotCanDisableIdleBackoff(t *testing.T) {
	cfg := NodeConfig{IdleSlotInterval: 0}

	if !shouldMineSlot(2, 0, cfg) {
		t.Fatal("zero idle interval should preserve mining every idle slot")
	}
}

func TestRewardPayoutAddressIsOptional(t *testing.T) {
	t.Setenv("POKOINPOS_REWARD_PAYOUT_ADDRESS", "")
	t.Setenv("POKOINPOS_LISTEN_PORT", "43000")
	t.Setenv("POKOINPOS_SLOT_SECONDS", "1")

	cfg, err := LoadNodeConfigFromEnv()
	if err != nil {
		t.Fatalf("empty payout address should be valid: %v", err)
	}
	if cfg.RewardPayoutAddress != "" {
		t.Fatalf("unexpected payout address: %q", cfg.RewardPayoutAddress)
	}
}

func TestRewardPayoutAddressValidation(t *testing.T) {
	t.Setenv("POKOINPOS_REWARD_PAYOUT_ADDRESS", "not-an-address")
	t.Setenv("POKOINPOS_LISTEN_PORT", "43000")
	t.Setenv("POKOINPOS_SLOT_SECONDS", "1")

	if _, err := LoadNodeConfigFromEnv(); err == nil {
		t.Fatal("expected invalid payout address to fail")
	}
}

func TestLoadNodeConfigUsesManifestDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"schemaVersion": 1,
			"network": {
				"bootstrapRefreshIntervalHours": 12
			},
			"evm": {
				"chainId": 26062026,
				"networkId": "26062026"
			},
			"bootstrap": {
				"fallbackPeers": [
					"92.5.153.117:43000",
					"130.162.242.213:43001"
				],
				"defaultJoinPeer": {
					"host": "92.5.153.117",
					"port": 43000
				}
			}
		}`))
	}))
	defer server.Close()

	t.Setenv("POKOINPOS_BOOTSTRAP_MANIFEST_URL", server.URL)
	t.Setenv("POKOINPOS_LISTEN_PORT", "43001")
	t.Setenv("POKOINPOS_ADVERTISE_HOST", "example.duckdns.com")

	cfg, err := LoadNodeConfigFromEnv()
	if err != nil {
		t.Fatalf("manifest defaults should produce valid config: %v", err)
	}
	if cfg.JoinHost != "92.5.153.117" || cfg.JoinPort != 43000 {
		t.Fatalf("unexpected join default: %s:%d", cfg.JoinHost, cfg.JoinPort)
	}
	if len(cfg.BootstrapPeers) != 2 {
		t.Fatalf("bootstrap peers: got %d want 2", len(cfg.BootstrapPeers))
	}
	if cfg.BootstrapRefreshHours != 12 {
		t.Fatalf("refresh hours: got %d want 12", cfg.BootstrapRefreshHours)
	}
	if cfg.EVMChainID != 26062026 || cfg.EVMNetworkID != "26062026" {
		t.Fatalf("unexpected evm defaults: %d %s", cfg.EVMChainID, cfg.EVMNetworkID)
	}
}
