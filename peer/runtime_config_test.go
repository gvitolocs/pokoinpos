package main

import "testing"

func TestMineAttemptsForNetworkScalesWithPendingTransactions(t *testing.T) {
	cfg := NodeConfig{
		BaseMineAttemptsPerTick:  1,
		MaxMineAttemptsPerTick:   100,
		MineAttemptsPerPendingTx: 25,
	}

	if got := mineAttemptsForNetwork(0, 1, cfg); got != 1 {
		t.Fatalf("idle attempts: got %d want 1", got)
	}
	if got := mineAttemptsForNetwork(2, 1, cfg); got != 51 {
		t.Fatalf("pending attempts: got %d want 51", got)
	}
	if got := mineAttemptsForNetwork(10, 1, cfg); got != 100 {
		t.Fatalf("capped attempts: got %d want 100", got)
	}
}

func TestMineAttemptsForNetworkDividesByAvailableNodes(t *testing.T) {
	cfg := NodeConfig{
		BaseMineAttemptsPerTick:  4,
		MaxMineAttemptsPerTick:   100,
		MineAttemptsPerPendingTx: 40,
	}

	if got := mineAttemptsForNetwork(1, 1, cfg); got != 44 {
		t.Fatalf("single-node attempts: got %d want 44", got)
	}
	if got := mineAttemptsForNetwork(1, 4, cfg); got != 11 {
		t.Fatalf("four-node attempts: got %d want 11", got)
	}
	if got := mineAttemptsForNetwork(1, 0, cfg); got != 44 {
		t.Fatalf("zero availability should fall back to one node: got %d want 44", got)
	}
}

func TestShouldMineSlotBacksOffWhenIdle(t *testing.T) {
	cfg := NodeConfig{IdleSlotInterval: 30}

	if !shouldMineSlot(1, 0, cfg) {
		t.Fatal("first idle slot should mine a keepalive block")
	}
	if shouldMineSlot(2, 0, cfg) {
		t.Fatal("idle slot before interval should not mine")
	}
	if !shouldMineSlot(30, 0, cfg) {
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
