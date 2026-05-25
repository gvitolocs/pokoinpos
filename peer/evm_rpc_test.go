package main

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"peer/account"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestHandleRPCSendRawTransaction(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	to := common.HexToAddress("0x2000000000000000000000000000000000000001")
	chainID := int64(26062026)

	peer := NewPeer(43099)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	rawTx := types.NewTransaction(0, to, new(big.Int).Mul(big.NewInt(10), account.EVMUnit), 21_000, big.NewInt(0), nil)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}

	server := &OpsServer{peer: peer, evmChainID: chainID, evmNetworkID: "26062026"}
	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendRawTransaction",
		Params:  []json.RawMessage{json.RawMessage(`"` + "0x" + common.Bytes2Hex(raw) + `"`)},
	})
	if rpcErr != nil {
		t.Fatalf("send raw tx returned rpc error: %+v", rpcErr)
	}
	if result != strings.ToLower(signed.Hash().Hex()) {
		t.Fatalf("wrong tx hash: got %v want %s", result, strings.ToLower(signed.Hash().Hex()))
	}
	if peer.MempoolSize() != 1 {
		t.Fatalf("expected tx in mempool, got %d", peer.MempoolSize())
	}
}

func TestHandleRPCAllowsZeroValueCancelTransaction(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	chainID := int64(26062026)

	peer := NewPeer(43100)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	rawTx := types.NewTransaction(0, from, big.NewInt(0), 21_000, big.NewInt(1_000_000_000), nil)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}

	server := &OpsServer{peer: peer, evmChainID: chainID, evmNetworkID: "26062026"}
	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendRawTransaction",
		Params:  []json.RawMessage{json.RawMessage(`"` + "0x" + common.Bytes2Hex(raw) + `"`)},
	})
	if rpcErr != nil {
		t.Fatalf("cancel tx returned rpc error: %+v", rpcErr)
	}
	if result != strings.ToLower(signed.Hash().Hex()) {
		t.Fatalf("wrong tx hash: got %v want %s", result, strings.ToLower(signed.Hash().Hex()))
	}
	if peer.PendingNonceOf(strings.ToLower(from.Hex())) != 1 {
		t.Fatalf("expected pending nonce to advance")
	}
}

func TestPeerEvictsMempoolTransactionInvalidatedByBestLedger(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	to := common.HexToAddress("0x3000000000000000000000000000000000000003")
	chainID := int64(26062026)

	peer := NewPeer(43104)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	rawTx := types.NewTransaction(0, to, new(big.Int).Mul(big.NewInt(10), account.EVMUnit), 21_000, big.NewInt(0), nil)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}
	tx, err := account.NewEVMTransaction("0x"+common.Bytes2Hex(raw), chainID)
	if err != nil {
		t.Fatalf("decode tx: %v", err)
	}
	peer.mempool[tx.ID] = *tx

	parent := peer.blockchain.BestLeafHash()
	block := account.NewCandidateBlock(miner, 1, parent, []account.SignedTransaction{*tx}, genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected block to apply")
	}

	peer.mempool[tx.ID] = *tx
	if removed := peer.pruneInvalidMempool("test"); removed != 1 {
		t.Fatalf("expected stale tx to be evicted, removed %d", removed)
	}
	if peer.MempoolSize() != 0 {
		t.Fatalf("expected empty mempool after eviction")
	}
}

func TestPeerPruneMempoolKeepsDependentEVMNonces(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	to := common.HexToAddress("0x4000000000000000000000000000000000000004")
	chainID := int64(26062026)

	peer := NewPeer(43105)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	for nonce := uint64(0); nonce < 2; nonce++ {
		rawTx := types.NewTransaction(nonce, to, new(big.Int).Mul(big.NewInt(10), account.EVMUnit), 21_000, big.NewInt(0), nil)
		signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
		if err != nil {
			t.Fatalf("sign tx %d: %v", nonce, err)
		}
		raw, err := signed.MarshalBinary()
		if err != nil {
			t.Fatalf("marshal tx %d: %v", nonce, err)
		}
		tx, err := account.NewEVMTransaction("0x"+common.Bytes2Hex(raw), chainID)
		if err != nil {
			t.Fatalf("decode tx %d: %v", nonce, err)
		}
		peer.mempool[tx.ID] = *tx
	}

	if removed := peer.pruneInvalidMempool("test"); removed != 0 {
		t.Fatalf("expected dependent nonce txs to stay, removed %d", removed)
	}
	if peer.MempoolSize() != 2 {
		t.Fatalf("expected both txs to stay in mempool")
	}
}

func TestHandleRPCFeeHistory(t *testing.T) {
	server := &OpsServer{peer: NewPeer(43101), evmChainID: 26062026, evmNetworkID: "26062026"}
	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_feeHistory",
		Params:  []json.RawMessage{json.RawMessage(`"0x2"`), json.RawMessage(`"latest"`), json.RawMessage(`[]`)},
	})
	if rpcErr != nil {
		t.Fatalf("fee history returned rpc error: %+v", rpcErr)
	}
	feeHistory, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("wrong fee history result type: %T", result)
	}
	baseFees, ok := feeHistory["baseFeePerGas"].([]string)
	if !ok {
		t.Fatalf("missing baseFeePerGas")
	}
	if len(baseFees) != 3 {
		t.Fatalf("wrong baseFeePerGas length: got %d want 3", len(baseFees))
	}
}

func TestHandleRPCBalanceUsesBestChainState(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	to := common.HexToAddress("0x3000000000000000000000000000000000000001")
	chainID := int64(26062026)

	peer := NewPeer(43102)
	peer.SetFinalityDepth(1)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	rawTx := types.NewTransaction(0, to, new(big.Int).Mul(big.NewInt(10), account.EVMUnit), 21_000, big.NewInt(1_000_000_000), nil)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}
	tx, err := account.NewEVMTransaction("0x"+common.Bytes2Hex(raw), chainID)
	if err != nil {
		t.Fatalf("decode tx: %v", err)
	}
	parent := peer.blockchain.BestLeafHash()
	block := account.NewCandidateBlock(miner, 1, parent, []account.SignedTransaction{*tx}, genesis.Seed)
	peer.ApplyBlock(block)

	server := &OpsServer{peer: peer, evmChainID: chainID, evmNetworkID: "26062026"}
	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getBalance",
		Params:  []json.RawMessage{json.RawMessage(`"` + from.Hex() + `"`), json.RawMessage(`"latest"`)},
	})
	if rpcErr != nil {
		t.Fatalf("balance returned rpc error: %+v", rpcErr)
	}
	if result != hexQuantity(pkToEVMValue(90)) {
		t.Fatalf("wrong best-chain balance: got %v want %s", result, hexQuantity(pkToEVMValue(90)))
	}
}

func TestHandleRPCDeploysAndCallsEVMContract(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	chainID := int64(26062026)

	peer := NewPeer(43103)
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[strings.ToLower(from.Hex())] = 100
	peer.ConfigurePoS(genesis, miner)

	// Init code returns runtime code that always returns uint256(42).
	initCode := common.FromHex("0x600a600c600039600a6000f3602a60005260206000f3")
	rawTx := types.NewContractCreation(0, big.NewInt(0), 500_000, big.NewInt(0), initCode)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(chainID)), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}
	tx, err := account.NewEVMTransaction("0x"+common.Bytes2Hex(raw), chainID)
	if err != nil {
		t.Fatalf("decode tx: %v", err)
	}
	parent := peer.blockchain.BestLeafHash()
	block := account.NewCandidateBlock(miner, 1, parent, []account.SignedTransaction{*tx}, genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected contract deployment block to apply")
	}

	contract := crypto.CreateAddress(from, 0)
	server := &OpsServer{peer: peer, evmChainID: chainID, evmNetworkID: "26062026"}
	code, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getCode",
		Params:  []json.RawMessage{json.RawMessage(`"` + contract.Hex() + `"`), json.RawMessage(`"latest"`)},
	})
	if rpcErr != nil {
		t.Fatalf("getCode returned rpc error: %+v", rpcErr)
	}
	if code != "0x602a60005260206000f3" {
		t.Fatalf("wrong runtime code: got %v", code)
	}

	result, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params:  []json.RawMessage{json.RawMessage(`{"from":"` + from.Hex() + `","to":"` + contract.Hex() + `","data":"0x"}`), json.RawMessage(`"latest"`)},
	})
	if rpcErr != nil {
		t.Fatalf("eth_call returned rpc error: %+v", rpcErr)
	}
	if result != "0x000000000000000000000000000000000000000000000000000000000000002a" {
		t.Fatalf("wrong call result: got %v", result)
	}

	receipt, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getTransactionReceipt",
		Params:  []json.RawMessage{json.RawMessage(`"` + tx.ID + `"`)},
	})
	if rpcErr != nil {
		t.Fatalf("receipt returned rpc error: %+v", rpcErr)
	}
	receiptMap, ok := receipt.(map[string]any)
	if !ok {
		t.Fatalf("wrong receipt type: %T", receipt)
	}
	if receiptMap["contractAddress"] != strings.ToLower(contract.Hex()) {
		t.Fatalf("wrong contract address in receipt: got %v", receiptMap["contractAddress"])
	}
	if receiptMap["gasUsed"] == "0x5208" {
		t.Fatalf("expected contract deployment gas, got transfer placeholder")
	}

	storage, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getStorageAt",
		Params:  []json.RawMessage{json.RawMessage(`"` + contract.Hex() + `"`), json.RawMessage(`"0x0"`), json.RawMessage(`"latest"`)},
	})
	if rpcErr != nil {
		t.Fatalf("getStorageAt returned rpc error: %+v", rpcErr)
	}
	if storage != "0x0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("wrong empty storage: got %v", storage)
	}

	logs, rpcErr := server.handleRPC(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getLogs",
		Params:  []json.RawMessage{json.RawMessage(`{"address":"` + contract.Hex() + `"}`)},
	})
	if rpcErr != nil {
		t.Fatalf("getLogs returned rpc error: %+v", rpcErr)
	}
	if _, ok := logs.([]any); !ok {
		t.Fatalf("wrong logs type: %T", logs)
	}
}
