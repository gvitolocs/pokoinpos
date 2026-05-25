package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"peer/account"
	"testing"
)

func makeNFTTestPeer(t *testing.T) (*Peer, *account.Account, *account.Account) {
	t.Helper()
	miner, _ := account.NewAccount()
	owner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner, owner}, 1_000_000, 1_000_000_000, 42)
	peer := NewPeer(43108)
	peer.ConfigurePoS(genesis, miner)
	return peer, miner, owner
}

func TestNativeNFTMintTransferAndReplay(t *testing.T) {
	peer, miner, owner := makeNFTTestPeer(t)
	mint := account.NewNFTMintTransaction("nft-mint-1", miner, "Cards", "Charizard-001", miner.SafeEncode(), "ipfs://metadata", "sha256:abc", "ipfs://image")
	block1 := account.NewCandidateBlock(miner, 1, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*mint}, peer.genesis.Seed)
	if !peer.ApplyBlock(block1) {
		t.Fatalf("expected mint block to apply")
	}
	transfer := account.NewNFTTransferTransaction("nft-transfer-1", miner, "Cards", "Charizard-001", owner.SafeEncode())
	block2 := account.NewCandidateBlock(miner, 2, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*transfer}, peer.genesis.Seed)
	if !peer.ApplyBlock(block2) {
		t.Fatalf("expected transfer block to apply")
	}

	token, exists := account.LedgerFromBlockchain(peer.blockchain, 0).NFT("cards", "charizard-001")
	if !exists {
		t.Fatalf("expected token to exist")
	}
	if token.Owner != owner.SafeEncode() {
		t.Fatalf("wrong owner: got %q want %q", token.Owner, owner.SafeEncode())
	}
	if token.MetadataURI != "ipfs://metadata" || token.ImageURI != "ipfs://image" {
		t.Fatalf("metadata did not replay: %+v", token)
	}
}

func TestNativeNFTRejectsDuplicateAndUnauthorizedTransfer(t *testing.T) {
	peer, miner, owner := makeNFTTestPeer(t)
	mint := account.NewNFTMintTransaction("nft-mint-1", miner, "cards", "001", owner.SafeEncode(), "", "", "")
	block1 := account.NewCandidateBlock(miner, 1, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*mint}, peer.genesis.Seed)
	if !peer.ApplyBlock(block1) {
		t.Fatalf("expected mint block to apply")
	}
	duplicate := account.NewNFTMintTransaction("nft-mint-2", miner, "cards", "001", owner.SafeEncode(), "", "", "")
	block2 := account.NewCandidateBlock(miner, 2, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*duplicate}, peer.genesis.Seed)
	if peer.ApplyBlock(block2) {
		t.Fatalf("duplicate mint should be rejected")
	}
	unauthorized := account.NewNFTTransferTransaction("nft-transfer-1", miner, "cards", "001", miner.SafeEncode())
	block3 := account.NewCandidateBlock(miner, 3, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*unauthorized}, peer.genesis.Seed)
	if peer.ApplyBlock(block3) {
		t.Fatalf("transfer by non-owner should be rejected")
	}
}

func TestAdminNFTMintEndpointQueuesTransaction(t *testing.T) {
	peer, _, owner := makeNFTTestPeer(t)
	server := &OpsServer{peer: peer, adminToken: "secret", evmChainID: 26062026, evmNetworkID: "26062026"}
	body := bytes.NewBufferString(`{"collectionId":"cards","tokenId":"001","owner":"` + owner.SafeEncode() + `","metadataUri":"ipfs://metadata","metadataHash":"sha256:abc","imageUri":"ipfs://image"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/nft/mint", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.nftMint(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("wrong status: got %d body %s", rec.Code, rec.Body.String())
	}
	if peer.MempoolSize() != 1 {
		t.Fatalf("expected nft mint tx in mempool")
	}
}

func TestChainNFTEndpoints(t *testing.T) {
	peer, miner, owner := makeNFTTestPeer(t)
	mint := account.NewNFTMintTransaction("nft-mint-1", miner, "cards", "001", owner.SafeEncode(), "ipfs://metadata", "sha256:abc", "ipfs://image")
	block := account.NewCandidateBlock(miner, 1, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*mint}, peer.genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected mint block to apply")
	}
	server := &OpsServer{peer: peer, evmChainID: 26062026, evmNetworkID: "26062026"}

	req := httptest.NewRequest(http.MethodGet, "/chain/nfts/cards/001", nil)
	rec := httptest.NewRecorder()
	server.chainNFTs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("wrong detail status: got %d body %s", rec.Code, rec.Body.String())
	}
	var token account.NFTToken
	if err := json.Unmarshal(rec.Body.Bytes(), &token); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if token.Owner != owner.SafeEncode() {
		t.Fatalf("wrong owner: %+v", token)
	}

	req = httptest.NewRequest(http.MethodGet, "/chain/nfts/owner/"+owner.SafeEncode(), nil)
	rec = httptest.NewRecorder()
	server.chainNFTs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("wrong owner status: got %d body %s", rec.Code, rec.Body.String())
	}
	var ownerResult struct {
		Count  int                `json:"count"`
		Tokens []account.NFTToken `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ownerResult); err != nil {
		t.Fatalf("decode owner result: %v", err)
	}
	if ownerResult.Count != 1 || len(ownerResult.Tokens) != 1 {
		t.Fatalf("wrong owner tokens: %+v", ownerResult)
	}
}
