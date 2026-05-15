package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"peer/account"
	"testing"
)

func makeExplorerTestServer(t *testing.T) (*OpsServer, *account.SignedTransaction) {
	t.Helper()
	miner, _ := account.NewAccount()
	from, _ := account.NewAccount()
	to, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner, from, to}, 1_000_000, 1_000_000_000, 42)
	peer := NewPeer(43104)
	peer.ConfigurePoS(genesis, miner)
	tx := account.NewSignedTransaction("explorer-tx-1", from, to.SafeEncode(), 10)
	parent := peer.blockchain.BestLeafHash()
	block := account.NewCandidateBlock(miner, 1, parent, []account.SignedTransaction{*tx}, genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected block to apply")
	}
	return &OpsServer{peer: peer, evmChainID: 26062026, evmNetworkID: "26062026"}, tx
}

func TestExplorerTxEndpoint(t *testing.T) {
	server, tx := makeExplorerTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/explorer/tx/"+tx.ID, nil)
	rec := httptest.NewRecorder()

	server.explorerTx(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("wrong status: got %d body %s", rec.Code, rec.Body.String())
	}
	var got ExplorerTx
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Hash != tx.ID || got.BlockNumber != 1 {
		t.Fatalf("wrong tx response: %+v", got)
	}
}

func TestHandleRPCTransactionByBlockNumberAndIndex(t *testing.T) {
	server, tx := makeExplorerTestServer(t)
	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getTransactionByBlockNumberAndIndex",
		Params:  []json.RawMessage{json.RawMessage(`"0x1"`), json.RawMessage(`"0x0"`)},
	})
	if rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	got, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("wrong result type: %T", result)
	}
	if got["hash"] != tx.ID {
		t.Fatalf("wrong tx hash: got %v want %s", got["hash"], tx.ID)
	}
}

func TestAdminMintEndpointRequiresAuthAndQueuesMint(t *testing.T) {
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
	peer := NewPeer(43105)
	peer.ConfigurePoS(genesis, miner)
	server := &OpsServer{peer: peer, adminToken: "secret", evmChainID: 26062026, evmNetworkID: "26062026"}

	body := bytes.NewBufferString(`{"to":"0x74466c3a204429b22ce8558f3f18f3c59f67fcb3","amount":2000000}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/mint", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.mint(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("wrong status: got %d body %s", rec.Code, rec.Body.String())
	}
	if peer.MempoolSize() != 1 {
		t.Fatalf("expected mint tx in mempool")
	}
}

func TestAdminMintPreservesValidatorAddressCase(t *testing.T) {
	miner, _ := account.NewAccount()
	validator, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
	peer := NewPeer(43106)
	peer.ConfigurePoS(genesis, miner)
	server := &OpsServer{peer: peer, adminToken: "secret", evmChainID: 26062026, evmNetworkID: "26062026"}

	to := validator.SafeEncode()
	body := bytes.NewBufferString(`{"to":"` + to + `","amount":100}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/mint", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.mint(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("wrong status: got %d body %s", rec.Code, rec.Body.String())
	}
	for _, pending := range peer.mempool {
		if pending.To != to {
			t.Fatalf("validator address case was changed: got %q want %q", pending.To, to)
		}
		return
	}
	t.Fatal("expected pending mint transaction")
}

func TestAdminWithdrawQueuesPayoutTransaction(t *testing.T) {
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
	peer := NewPeer(43107)
	peer.ConfigurePoS(genesis, miner)
	server := &OpsServer{peer: peer, adminToken: "secret", evmChainID: 26062026, evmNetworkID: "26062026"}

	body := bytes.NewBufferString(`{"to":"0x1111111111111111111111111111111111111111","amount":100}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/withdraw", body)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	server.withdraw(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("wrong status: got %d body %s", rec.Code, rec.Body.String())
	}
	for _, pending := range peer.mempool {
		if pending.From != miner.SafeEncode() {
			t.Fatalf("withdraw should come from validator miner: got %q", pending.From)
		}
		if pending.To != "0x1111111111111111111111111111111111111111" {
			t.Fatalf("wrong payout address: %q", pending.To)
		}
		return
	}
	t.Fatal("expected pending withdraw transaction")
}
