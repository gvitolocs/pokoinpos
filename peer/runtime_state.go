package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"peer/account"
)

type nodeState struct {
	Genesis  account.GenesisMetaData    `json:"genesis"`
	Miner    account.AccountKeyMaterial `json:"miner"`
	Blocks   []account.Block            `json:"blocks"`
	LastSlot int                        `json:"lastSlot"`
}

func stateFilePath(dir string, listenPort int) string {
	return filepath.Join(dir, fmt.Sprintf("node-%d-state.json", listenPort))
}

func loadNodeState(dir string, listenPort int) (*nodeState, error) {
	path := stateFilePath(dir, listenPort)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state nodeState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if len(state.Blocks) == 0 {
		return nil, fmt.Errorf("state file has no blocks")
	}
	return &state, nil
}

func saveNodeState(dir string, listenPort int, peer *Peer, lastSlot int) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	if peer.blockchain == nil || peer.genesis == nil || peer.miner == nil {
		return fmt.Errorf("peer state not initialized")
	}
	state := nodeState{
		Genesis:  *peer.genesis,
		Miner:    peer.miner.KeyMaterial(),
		Blocks:   peer.blockchain.BlocksSnapshot(),
		LastSlot: lastSlot,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := stateFilePath(dir, listenPort)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
