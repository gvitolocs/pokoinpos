package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type NodeConfig struct {
	RunMode                  string
	ListenPort               int
	JoinHost                 string
	JoinPort                 int
	OpsAddr                  string
	AdminToken               string
	SlotSeconds              int
	GenesisHardness          int
	GenesisSeed              int
	InitialBalance           int
	StateDir                 string
	StateSaveIntervalSeconds int
	ReconnectIntervalSeconds int
	FinalityDepth            int
	EVMChainID               int64
	EVMNetworkID             string
	EVMGenesisAlloc          map[string]int
}

func LoadNodeConfigFromEnv() (NodeConfig, error) {
	cfg := NodeConfig{
		RunMode:         envOrDefault("POKOINPOS_RUN_MODE", "demo"),
		ListenPort:      envIntOrDefault("POKOINPOS_LISTEN_PORT", 43000),
		JoinHost:        envOrDefault("POKOINPOS_JOIN_HOST", "127.0.0.1"),
		JoinPort:        envIntOrDefault("POKOINPOS_JOIN_PORT", -1),
		OpsAddr:         envOrDefault("POKOINPOS_OPS_ADDR", ":8080"),
		AdminToken:      envOrDefault("POKOINPOS_ADMIN_TOKEN", ""),
		SlotSeconds:     envIntOrDefault("POKOINPOS_SLOT_SECONDS", 1),
		GenesisHardness: envIntOrDefault("POKOINPOS_GENESIS_HARDNESS", 10000),
		GenesisSeed:     envIntOrDefault("POKOINPOS_GENESIS_SEED", 42),
		InitialBalance:  envIntOrDefault("POKOINPOS_INITIAL_BALANCE", 1000000),
		StateDir:        envOrDefault("POKOINPOS_STATE_DIR", "/data"),
		StateSaveIntervalSeconds: envIntOrDefault(
			"POKOINPOS_STATE_SAVE_INTERVAL_SECONDS",
			15,
		),
		ReconnectIntervalSeconds: envIntOrDefault(
			"POKOINPOS_RECONNECT_INTERVAL_SECONDS",
			5,
		),
		FinalityDepth: envIntOrDefault("POKOINPOS_FINALITY_DEPTH", 1),
		EVMChainID:    envInt64OrDefault("POKOINPOS_EVM_CHAIN_ID", 26062026),
		EVMNetworkID:  envOrDefault("POKOINPOS_EVM_NETWORK_ID", "26062026"),
		EVMGenesisAlloc: envAllocOrDefault(
			"POKOINPOS_EVM_GENESIS_ALLOC",
			map[string]int{},
		),
	}
	if cfg.ListenPort <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_LISTEN_PORT must be > 0")
	}
	if cfg.SlotSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_SLOT_SECONDS must be > 0")
	}
	if !strings.HasPrefix(cfg.OpsAddr, ":") && !strings.Contains(cfg.OpsAddr, ":") {
		return cfg, fmt.Errorf("POKOINPOS_OPS_ADDR must be host:port or :port")
	}
	if cfg.StateSaveIntervalSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_STATE_SAVE_INTERVAL_SECONDS must be > 0")
	}
	if cfg.ReconnectIntervalSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_RECONNECT_INTERVAL_SECONDS must be > 0")
	}
	if cfg.FinalityDepth < 0 {
		return cfg, fmt.Errorf("POKOINPOS_FINALITY_DEPTH must be >= 0")
	}
	if cfg.EVMChainID <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_EVM_CHAIN_ID must be > 0")
	}
	if strings.TrimSpace(cfg.EVMNetworkID) == "" {
		return cfg, fmt.Errorf("POKOINPOS_EVM_NETWORK_ID must not be empty")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envInt64OrDefault(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envAllocOrDefault(key string, fallback map[string]int) map[string]int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	alloc := make(map[string]int)
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		address := strings.ToLower(strings.TrimSpace(parts[0]))
		amount, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || address == "" || amount < 0 {
			continue
		}
		alloc[address] = amount
	}
	return alloc
}
