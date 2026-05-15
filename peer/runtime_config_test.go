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
