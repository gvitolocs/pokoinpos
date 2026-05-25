package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"peer/account"
)

func TestLateJoiningPeerReceivesPersistedHistoricalBlocks(t *testing.T) {
	stateBytes, err := os.ReadFile("/tmp/pokoin-peer1-state.json")
	if err != nil {
		t.Skip("peer1 state fixture not available")
	}
	var restored nodeState
	if err := json.Unmarshal(stateBytes, &restored); err != nil {
		t.Fatalf("decode state fixture: %v", err)
	}
	if len(restored.Blocks) < 2 {
		t.Skip("state fixture does not contain historical blocks")
	}
	port1 := getFreePort(t)
	port2 := getFreePort(t)

	miner2, _ := account.NewAccount()
	bootstrapGenesis := restored.Genesis
	localGenesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner2}, 1_000_000, 1_000_000_000, 7)

	peer1 := NewPeer(port1)
	miner1, err := account.AccountFromKeyMaterial(restored.Miner)
	if err != nil {
		t.Fatalf("restore miner: %v", err)
	}
	peer1.ConfigurePoS(&bootstrapGenesis, miner1)
	peer1.blockchain.ReplaceBlocks(restored.Blocks)
	peer1.ledger = account.LedgerFromBlockchain(peer1.blockchain, peer1.finalityDepth)

	// Simulate a node restored from disk: it has historical blocks in blockchain
	// state, but msgHistory is empty because flood history is process-local RAM.
	if got := peer1.ChainHeight(); got < 2 {
		t.Fatalf("setup failed: peer1 height=%d", got)
	}
	if err := peer1.Start(); err != nil {
		t.Fatalf("start peer1: %v", err)
	}

	peer2 := NewPeer(port2)
	peer2.ConfigurePoS(localGenesis, miner2)
	if err := peer2.Start(); err != nil {
		t.Fatalf("start peer2: %v", err)
	}
	if _, err := peer2.Connect("127.0.0.1", port1); err != nil {
		t.Fatalf("connect peer2 to peer1: %v", err)
	}

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("historical sync did not complete: peer2 height=%d peer1 height=%d", peer2.ChainHeight(), peer1.ChainHeight())
		case <-ticker.C:
			if peer2.ChainHeight() == peer1.ChainHeight() {
				return
			}
		}
	}
}

func TestHistoricalSyncRejectsGenesisMismatchAfterLocalHistory(t *testing.T) {
	miner1, _ := account.NewAccount()
	miner2, _ := account.NewAccount()
	genesis1 := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner1}, 1_000_000, 1_000_000_000, 42)
	genesis2 := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner2}, 1_000_000, 1_000_000_000, 43)

	peer := NewPeer(getFreePort(t))
	peer.ConfigurePoS(genesis1, miner1)
	localBlock := account.NewCandidateBlock(miner1, 1, peer.blockchain.BestLeafHash(), nil, genesis1.Seed)
	if ok := peer.blockchain.AddBlock(localBlock); !ok {
		t.Fatal("setup failed: expected local block to be valid")
	}
	peer.ledger = account.LedgerFromBlockchain(peer.blockchain, peer.finalityDepth)

	remoteChain := account.NewBlockchainWithGenesis(genesis2)
	remoteBlock := account.NewCandidateBlock(miner2, 1, remoteChain.BestLeafHash(), nil, genesis2.Seed)
	if ok := remoteChain.AddBlock(remoteBlock); !ok {
		t.Fatal("setup failed: expected remote block to be valid")
	}
	resp := chainSyncResponse{
		GenesisHash: genesisContentHash(remoteChain.CanonicalBlocks(0)[0]),
		Height:      remoteChain.BestHeight(),
		Blocks:      remoteChain.CanonicalBlocks(0),
	}
	raw, _ := json.Marshal(resp)
	if peer.handleChainSyncResponse(&Message{Type: "ChainSyncResponse", From: "remote", Payload: raw}) {
		t.Fatal("expected genesis mismatch to be rejected for peer with local history")
	}
	if got := peer.ChainHeight(); got != 1 {
		t.Fatalf("local chain was modified despite genesis mismatch: height=%d", got)
	}
}

func TestHistoricalSyncRefusesFullReplacementAfterLocalHistory(t *testing.T) {
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	peer := NewPeer(getFreePort(t))
	peer.ConfigurePoS(genesis, miner)

	shortFork := account.NewCandidateBlock(miner, 1, peer.blockchain.BestLeafHash(), nil, genesis.Seed)
	if ok := peer.blockchain.AddBlock(shortFork); !ok {
		t.Fatal("setup failed: expected local short fork block")
	}
	peer.ledger = account.LedgerFromBlockchain(peer.blockchain, peer.finalityDepth)

	remote := account.NewBlockchainWithGenesis(genesis)
	first := account.NewCandidateBlock(miner, 2, remote.BestLeafHash(), nil, genesis.Seed)
	if ok := remote.AddBlock(first); !ok {
		t.Fatal("setup failed: expected remote first block")
	}
	second := account.NewCandidateBlock(miner, 3, remote.BestLeafHash(), nil, genesis.Seed)
	if ok := remote.AddBlock(second); !ok {
		t.Fatal("setup failed: expected remote second block")
	}
	remoteBlocks := remote.CanonicalBlocks(0)
	resp := chainSyncResponse{
		GenesisHash: genesisContentHash(remoteBlocks[0]),
		Height:      remote.BestHeight(),
		Blocks:      remoteBlocks,
	}
	raw, _ := json.Marshal(resp)
	if peer.handleChainSyncResponse(&Message{Type: "ChainSyncResponse", From: "remote", Payload: raw}) {
		t.Fatal("expected full canonical chain replacement to be refused after local history")
	}
	if got := peer.ChainHeight(); got != 1 {
		t.Fatalf("local chain was modified despite replacement refusal: height=%d", got)
	}
}

func TestHistoricalSyncSwitchesToLongerForkWithCommonAncestor(t *testing.T) {
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	peer := NewPeer(getFreePort(t))
	peer.ConfigurePoS(genesis, miner)

	common := account.NewCandidateBlock(miner, 1, peer.blockchain.BestLeafHash(), nil, genesis.Seed)
	if ok := peer.blockchain.AddBlock(common); !ok {
		t.Fatal("setup failed: expected common block")
	}
	localFork := account.NewCandidateBlock(miner, 2, peer.blockchain.BestLeafHash(), nil, genesis.Seed)
	if ok := peer.blockchain.AddBlock(localFork); !ok {
		t.Fatal("setup failed: expected local fork block")
	}
	peer.ledger = account.LedgerFromBlockchain(peer.blockchain, peer.finalityDepth)

	remote := account.NewBlockchainWithGenesis(genesis)
	if ok := remote.AddBlock(common); !ok {
		t.Fatal("setup failed: expected remote common block")
	}
	remoteFork1 := account.NewCandidateBlock(miner, 3, remote.BestLeafHash(), nil, genesis.Seed)
	if ok := remote.AddBlock(remoteFork1); !ok {
		t.Fatal("setup failed: expected remote fork block")
	}
	remoteFork2 := account.NewCandidateBlock(miner, 4, remote.BestLeafHash(), nil, genesis.Seed)
	if ok := remote.AddBlock(remoteFork2); !ok {
		t.Fatal("setup failed: expected remote longer fork block")
	}

	remoteBlocks := remote.CanonicalBlocks(0)
	resp := chainSyncResponse{
		GenesisHash: genesisContentHash(remoteBlocks[0]),
		Height:      remote.BestHeight(),
		Blocks:      remoteBlocks,
	}
	raw, _ := json.Marshal(resp)
	if !peer.handleChainSyncResponse(&Message{Type: "ChainSyncResponse", From: "remote", Payload: raw}) {
		t.Fatal("expected longer fork with common ancestor to be accepted")
	}
	if got, want := peer.blockchain.BestLeafHash(), remote.BestLeafHash(); got != want {
		t.Fatalf("wrong best leaf after fork switch: got %s want %s", got, want)
	}
}
