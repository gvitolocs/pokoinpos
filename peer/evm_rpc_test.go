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
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000, 42)
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
