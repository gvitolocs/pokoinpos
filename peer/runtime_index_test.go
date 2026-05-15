package main

import (
	"peer/account"
	"strings"
	"testing"
)

func TestBuildExplorerIndexIndexesCanonicalBlocksAndAddresses(t *testing.T) {
	miner, _ := account.NewAccount()
	from, _ := account.NewAccount()
	to, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner, from, to}, 1_000_000, 1_000_000, 42)
	peer := NewPeer(43103)
	peer.SetFinalityDepth(1)
	peer.ConfigurePoS(genesis, miner)

	tx := account.NewSignedTransaction("idx-tx-1", from, to.SafeEncode(), 10)
	parent := peer.blockchain.BestLeafHash()
	block := account.NewCandidateBlock(miner, 1, parent, []account.SignedTransaction{*tx}, genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected block to apply")
	}

	idx := BuildExplorerIndex(peer)
	if idx.LatestHeight != 1 {
		t.Fatalf("wrong latest height: got %d want 1", idx.LatestHeight)
	}
	if idx.FinalizedHeight != 0 {
		t.Fatalf("wrong finalized height: got %d want 0", idx.FinalizedHeight)
	}
	explorerTx, exists := idx.TxsByHash["idx-tx-1"]
	if !exists {
		t.Fatalf("expected transaction in index")
	}
	if explorerTx.BlockNumber != 1 || explorerTx.TransactionIndex != 0 {
		t.Fatalf("wrong tx location: block=%d index=%d", explorerTx.BlockNumber, explorerTx.TransactionIndex)
	}
	if len(idx.TxsByAddress[strings.ToLower(from.SafeEncode())]) != 1 {
		t.Fatalf("expected sender address history")
	}
	if len(idx.TxsByAddress[strings.ToLower(to.SafeEncode())]) != 1 {
		t.Fatalf("expected receiver address history")
	}
}
