package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type NodeConfig struct {
	RunMode                   string
	ListenPort                int
	AdvertiseHost             string
	JoinHost                  string
	JoinPort                  int
	BootstrapPeers            []PeerAddress
	BootstrapManifestURL      string
	BootstrapRefreshHours     int
	OpsAddr                   string
	AdminToken                string
	RewardPayoutAddress       string
	ReservedSupplyAddresses   []string
	SlotSeconds               int
	IdleSlotInterval          int
	BaseMineAttemptsPerTick   int
	MaxMineAttemptsPerTick    int
	MineAttemptsPerPendingTx  int
	DynamicHardnessForkHeight int
	DynamicHardnessMin        int
	DynamicHardnessMax        int
	DynamicHardnessWindow     int
	FastBlockTargetSlots      int
	IdleBlockTargetSlots      int
	GenesisHardness           int
	GenesisSeed               int
	InitialBalance            int
	StateDir                  string
	StateSaveIntervalSeconds  int
	ReconnectIntervalSeconds  int
	FinalityDepth             int
	EVMChainID                int64
	EVMNetworkID              string
	EVMGenesisAlloc           map[string]int
}

func LoadNodeConfigFromEnv() (NodeConfig, error) {
	manifestURL := envOrDefault("POKOINPOS_BOOTSTRAP_MANIFEST_URL", "https://pokoin.com/bootstrap-peers.json")
	var manifest bootstrapManifest
	if strings.TrimSpace(os.Getenv("POKOINPOS_BOOTSTRAP_MANIFEST_URL")) != "" {
		var manifestErr error
		manifest, manifestErr = FetchBootstrapManifestDefaults(context.Background(), manifestURL)
		if manifestErr != nil {
			logEvent("bootstrap_manifest_defaults_unavailable", map[string]any{
				"url":   manifestURL,
				"error": manifestErr.Error(),
			})
		}
	}
	defaultJoinHost := "127.0.0.1"
	defaultJoinPort := -1
	if manifest.Bootstrap.DefaultJoinPeer.Host != "" && manifest.Bootstrap.DefaultJoinPeer.Port > 0 {
		defaultJoinHost = manifest.Bootstrap.DefaultJoinPeer.Host
		defaultJoinPort = manifest.Bootstrap.DefaultJoinPeer.Port
	}
	defaultBootstrapPeers := parseBootstrapPeers(strings.Join(manifestFallbackPeers(manifest), ","))
	defaultBootstrapRefreshHours := manifest.Network.BootstrapRefreshIntervalHours
	if defaultBootstrapRefreshHours <= 0 {
		defaultBootstrapRefreshHours = 24
	}
	defaultEVMChainID := manifest.EVM.ChainID
	if defaultEVMChainID <= 0 {
		defaultEVMChainID = 26062026
	}
	defaultEVMNetworkID := strings.TrimSpace(manifest.EVM.NetworkID)
	if defaultEVMNetworkID == "" {
		defaultEVMNetworkID = "26062026"
	}
	joinHost := envOrDefault("POKOINPOS_JOIN_HOST", defaultJoinHost)
	joinPort := envIntOrDefault("POKOINPOS_JOIN_PORT", defaultJoinPort)
	cfg := NodeConfig{
		RunMode:    envOrDefault("POKOINPOS_RUN_MODE", "demo"),
		ListenPort: envIntOrDefault("POKOINPOS_LISTEN_PORT", 43000),
		AdvertiseHost: envOrDefault(
			"POKOINPOS_ADVERTISE_HOST",
			joinHost,
		),
		JoinHost: joinHost,
		JoinPort: joinPort,
		BootstrapPeers: envBootstrapPeersOrDefaultWithFallback(
			"POKOINPOS_BOOTSTRAP_PEERS",
			joinHost,
			joinPort,
			defaultBootstrapPeers,
		),
		BootstrapManifestURL:  manifestURL,
		BootstrapRefreshHours: envIntOrDefault("POKOINPOS_BOOTSTRAP_REFRESH_INTERVAL_HOURS", defaultBootstrapRefreshHours),
		OpsAddr:               envOrDefault("POKOINPOS_OPS_ADDR", ":8080"),
		AdminToken: envOrDefault(
			"POKOINPOS_OPERATOR_TOKEN",
			envOrDefault("POKOINPOS_ADMIN_TOKEN", ""),
		),
		RewardPayoutAddress: strings.ToLower(
			envOrDefault("POKOINPOS_REWARD_PAYOUT_ADDRESS", ""),
		),
		ReservedSupplyAddresses: envCSVOrDefault(
			"POKOINPOS_RESERVED_SUPPLY_ADDRESSES",
			[]string{"0x74466c3a204429b22ce8558f3f18f3c59f67fcb3"},
		),
		SlotSeconds:      envIntOrDefault("POKOINPOS_SLOT_SECONDS", 1),
		IdleSlotInterval: envIntOrDefault("POKOINPOS_IDLE_SLOT_INTERVAL", 300),
		BaseMineAttemptsPerTick: envIntOrDefault(
			"POKOINPOS_BASE_MINE_ATTEMPTS_PER_TICK",
			1,
		),
		MaxMineAttemptsPerTick: envIntOrDefault(
			"POKOINPOS_MAX_MINE_ATTEMPTS_PER_TICK",
			envIntOrDefault("POKOINPOS_MINE_ATTEMPTS_PER_TICK", 5),
		),
		MineAttemptsPerPendingTx: envIntOrDefault(
			"POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX",
			1,
		),
		DynamicHardnessForkHeight: envIntOrDefault("POKOINPOS_DYNAMIC_HARDNESS_FORK_HEIGHT", 0),
		DynamicHardnessMin:        envIntOrDefault("POKOINPOS_DYNAMIC_HARDNESS_MIN", 100),
		DynamicHardnessMax:        envIntOrDefault("POKOINPOS_DYNAMIC_HARDNESS_MAX", 10000000),
		DynamicHardnessWindow:     envIntOrDefault("POKOINPOS_DYNAMIC_HARDNESS_WINDOW", 32),
		FastBlockTargetSlots:      envIntOrDefault("POKOINPOS_FAST_BLOCK_TARGET_SLOTS", 2),
		IdleBlockTargetSlots:      envIntOrDefault("POKOINPOS_IDLE_BLOCK_TARGET_SLOTS", 20),
		GenesisHardness:           envIntOrDefault("POKOINPOS_GENESIS_HARDNESS", 10000),
		GenesisSeed:               envIntOrDefault("POKOINPOS_GENESIS_SEED", 42),
		InitialBalance:            envIntOrDefault("POKOINPOS_INITIAL_BALANCE", 1000000),
		StateDir:                  envOrDefault("POKOINPOS_STATE_DIR", "/data"),
		StateSaveIntervalSeconds: envIntOrDefault(
			"POKOINPOS_STATE_SAVE_INTERVAL_SECONDS",
			15,
		),
		ReconnectIntervalSeconds: envIntOrDefault(
			"POKOINPOS_RECONNECT_INTERVAL_SECONDS",
			5,
		),
		FinalityDepth: envIntOrDefault("POKOINPOS_FINALITY_DEPTH", 1),
		EVMChainID:    envInt64OrDefault("POKOINPOS_EVM_CHAIN_ID", defaultEVMChainID),
		EVMNetworkID:  envOrDefault("POKOINPOS_EVM_NETWORK_ID", defaultEVMNetworkID),
		EVMGenesisAlloc: envAllocOrDefault(
			"POKOINPOS_EVM_GENESIS_ALLOC",
			map[string]int{},
		),
	}
	if cfg.ListenPort <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_LISTEN_PORT must be > 0")
	}
	if strings.TrimSpace(cfg.AdvertiseHost) == "" {
		return cfg, fmt.Errorf("POKOINPOS_ADVERTISE_HOST must not be empty")
	}
	if cfg.SlotSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_SLOT_SECONDS must be > 0")
	}
	if cfg.IdleSlotInterval < 0 {
		return cfg, fmt.Errorf("POKOINPOS_IDLE_SLOT_INTERVAL must be >= 0")
	}
	if cfg.BaseMineAttemptsPerTick <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_BASE_MINE_ATTEMPTS_PER_TICK must be > 0")
	}
	if cfg.MaxMineAttemptsPerTick <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_MAX_MINE_ATTEMPTS_PER_TICK must be > 0")
	}
	if cfg.MineAttemptsPerPendingTx < 0 {
		return cfg, fmt.Errorf("POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX must be >= 0")
	}
	if cfg.DynamicHardnessForkHeight < 0 {
		return cfg, fmt.Errorf("POKOINPOS_DYNAMIC_HARDNESS_FORK_HEIGHT must be >= 0")
	}
	if cfg.DynamicHardnessMin <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_DYNAMIC_HARDNESS_MIN must be > 0")
	}
	if cfg.DynamicHardnessMax < cfg.DynamicHardnessMin {
		return cfg, fmt.Errorf("POKOINPOS_DYNAMIC_HARDNESS_MAX must be >= POKOINPOS_DYNAMIC_HARDNESS_MIN")
	}
	if cfg.DynamicHardnessWindow <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_DYNAMIC_HARDNESS_WINDOW must be > 0")
	}
	if cfg.FastBlockTargetSlots <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_FAST_BLOCK_TARGET_SLOTS must be > 0")
	}
	if cfg.IdleBlockTargetSlots <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_IDLE_BLOCK_TARGET_SLOTS must be > 0")
	}
	if !strings.HasPrefix(cfg.OpsAddr, ":") && !strings.Contains(cfg.OpsAddr, ":") {
		return cfg, fmt.Errorf("POKOINPOS_OPS_ADDR must be host:port or :port")
	}
	if cfg.RewardPayoutAddress != "" && (!strings.HasPrefix(cfg.RewardPayoutAddress, "0x") || len(cfg.RewardPayoutAddress) != 42) {
		return cfg, fmt.Errorf("POKOINPOS_REWARD_PAYOUT_ADDRESS must be an EVM 0x address")
	}
	if cfg.StateSaveIntervalSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_STATE_SAVE_INTERVAL_SECONDS must be > 0")
	}
	if cfg.ReconnectIntervalSeconds <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_RECONNECT_INTERVAL_SECONDS must be > 0")
	}
	if cfg.BootstrapRefreshHours <= 0 {
		return cfg, fmt.Errorf("POKOINPOS_BOOTSTRAP_REFRESH_INTERVAL_HOURS must be > 0")
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

func envCSVOrDefault(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(item))
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return fallback
	}
	return values
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

func envBootstrapPeersOrDefault(key string, joinHost string, joinPort int) []PeerAddress {
	return envBootstrapPeersOrDefaultWithFallback(key, joinHost, joinPort, nil)
}

func envBootstrapPeersOrDefaultWithFallback(key string, joinHost string, joinPort int, fallback []PeerAddress) []PeerAddress {
	raw := strings.TrimSpace(os.Getenv(key))
	peers := parseBootstrapPeers(raw)
	if len(peers) > 0 {
		return peers
	}
	if len(fallback) > 0 {
		return append([]PeerAddress(nil), fallback...)
	}
	if joinPort > 0 && strings.TrimSpace(joinHost) != "" {
		return []PeerAddress{{
			ID:   strconv.Itoa(joinPort),
			Host: strings.TrimSpace(joinHost),
			Port: joinPort,
		}}
	}
	return nil
}

func parseBootstrapPeers(raw string) []PeerAddress {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	peers := make([]PeerAddress, 0)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		host, portText, err := splitHostPortLenient(entry)
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port <= 0 {
			continue
		}
		peers = append(peers, PeerAddress{
			ID:   strconv.Itoa(port),
			Host: host,
			Port: port,
		})
	}
	return peers
}

func splitHostPortLenient(value string) (string, string, error) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("missing port")
	}
	port := parts[len(parts)-1]
	host := strings.Join(parts[:len(parts)-1], ":")
	host = strings.Trim(host, "[]")
	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return "", "", fmt.Errorf("invalid host port")
	}
	return strings.TrimSpace(host), strings.TrimSpace(port), nil
}
