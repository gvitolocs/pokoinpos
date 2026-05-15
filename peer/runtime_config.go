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
