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

	rawTx := types.NewTransaction(0, to, big.NewInt(10), 21_000, big.NewInt(0), nil)
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
