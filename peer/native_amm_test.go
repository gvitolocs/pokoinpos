package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"peer/account"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func makeAMMTestPeer(t *testing.T) (*Peer, *account.Account, *account.Account, string) {
	t.Helper()
	miner, _ := account.NewAccount()
	user, _ := account.NewAccount()
	userAddress := "0x1111111111111111111111111111111111111111"
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner, user}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[userAddress] = 10_000
	peer := NewPeer(43109)
	peer.ConfigurePoS(genesis, miner)
	return peer, miner, user, userAddress
}

func makeAMMEVMTestPeer(t *testing.T) (*Peer, *account.Account, *ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate evm key: %v", err)
	}
	userAddress := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	miner, _ := account.NewAccount()
	genesis := account.MakeGenesisMetaDataFromAccounts([]*account.Account{miner}, 1_000_000, 1_000_000_000, 42)
	genesis.InitialBalances[userAddress] = 10_000
	peer := NewPeer(43110)
	peer.ConfigurePoS(genesis, miner)
	return peer, miner, key, userAddress
}

func applyAMMBlock(t *testing.T, peer *Peer, miner *account.Account, txs ...*account.SignedTransaction) {
	t.Helper()
	values := make([]account.SignedTransaction, 0, len(txs))
	for _, tx := range txs {
		values = append(values, *tx)
	}
	block := account.NewCandidateBlock(miner, peer.ChainHeight()+1, peer.blockchain.BestLeafHash(), values, peer.genesis.Seed)
	if !peer.ApplyBlock(block) {
		t.Fatalf("expected AMM block to apply with txs: %+v", values)
	}
}

func TestNativeAMMPoolLiquiditySwapAndReplay(t *testing.T) {
	peer, miner, key, userAddress := makeAMMEVMTestPeer(t)
	poolID := account.PoolID(account.AssetPKN, account.AssetWPKN)
	applyAMMBlock(t, peer, miner,
		account.NewAssetCreditTransaction("credit-miner-wpkn", miner, account.AssetWPKN, miner.SafeEncode(), 10_000),
		account.NewAssetCreditTransaction("credit-user-wpkn", miner, account.AssetWPKN, userAddress, 1_000),
		account.NewAMMCreatePoolTransaction("create-pool", miner, account.AssetPKN, account.AssetWPKN),
		account.NewAMMAddLiquidityTransaction("add-liquidity", miner, poolID, 5_000, 5_000),
	)
	ledger := account.LedgerFromBlockchain(peer.blockchain, 0)
	pool, exists := ledger.AMMPool(poolID)
	if !exists {
		t.Fatalf("expected pool")
	}
	if pool.ReserveA != 5_000 || pool.ReserveB != 5_000 {
		t.Fatalf("wrong reserves: %+v", pool)
	}
	out, ok := account.QuoteSwap(pool, account.AssetPKN, 100)
	if !ok || out < 1 {
		t.Fatalf("expected quote, got %d ok=%v", out, ok)
	}
	swap := signedSwapTxWithKey(t, key, 0, poolID, account.AssetPKN, account.AssetWPKN, 100, out)
	applyAMMBlock(t, peer, miner, swap)
	ledger = account.LedgerFromBlockchain(peer.blockchain, 0)
	if ledger.GetBalance(userAddress) != 9_900 {
		t.Fatalf("wrong user pkn after swap: %d", ledger.GetBalance(userAddress))
	}
	if ledger.GetAssetBalance(account.AssetWPKN, userAddress) != 1_000+out {
		t.Fatalf("wrong user wpkn after swap: %d want %d", ledger.GetAssetBalance(account.AssetWPKN, userAddress), 1_000+out)
	}
}

func TestNativeAMMRejectsSlippageAndInsufficientAsset(t *testing.T) {
	peer, miner, key, _ := makeAMMEVMTestPeer(t)
	poolID := account.PoolID(account.AssetPKN, account.AssetWPKN)
	applyAMMBlock(t, peer, miner,
		account.NewAssetCreditTransaction("credit-miner-wpkn", miner, account.AssetWPKN, miner.SafeEncode(), 10_000),
		account.NewAMMCreatePoolTransaction("create-pool", miner, account.AssetPKN, account.AssetWPKN),
		account.NewAMMAddLiquidityTransaction("add-liquidity", miner, poolID, 5_000, 5_000),
	)
	badSlippage := signedSwapTxWithKey(t, key, 0, poolID, account.AssetPKN, account.AssetWPKN, 100, 10_000)
	block := account.NewCandidateBlock(miner, 2, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*badSlippage}, peer.genesis.Seed)
	if peer.ApplyBlock(block) {
		t.Fatalf("swap with impossible min amount should be rejected")
	}
	insufficient := signedSwapTxWithKey(t, key, 0, poolID, account.AssetWPKN, account.AssetPKN, 1, 1)
	block = account.NewCandidateBlock(miner, 2, peer.blockchain.BestLeafHash(), []account.SignedTransaction{*insufficient}, peer.genesis.Seed)
	if peer.ApplyBlock(block) {
		t.Fatalf("swap with no internal asset balance should be rejected")
	}
}

func TestHandleRPCSignedPokoinSwapTransaction(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	from := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	peer, miner, _, _ := makeAMMTestPeer(t)
	peer.genesis.InitialBalances[from] = 10_000
	peer.ConfigurePoS(peer.genesis, miner)
	poolID := account.PoolID(account.AssetPKN, account.AssetWPKN)
	applyAMMBlock(t, peer, miner,
		account.NewAssetCreditTransaction("credit-miner-wpkn", miner, account.AssetWPKN, miner.SafeEncode(), 10_000),
		account.NewAMMCreatePoolTransaction("create-pool", miner, account.AssetPKN, account.AssetWPKN),
		account.NewAMMAddLiquidityTransaction("add-liquidity", miner, poolID, 5_000, 5_000),
	)
	payload := &account.AMMPayload{Action: "amm_swap", PoolID: poolID, AssetIn: account.AssetPKN, AssetOut: account.AssetWPKN, AmountIn: 100, MinAmountOut: 1}
	rawTx := types.NewTransaction(0, common.HexToAddress(account.PokoinSwapRouterAddress), big.NewInt(0), 120_000, big.NewInt(1_000_000_000), account.EncodePokoinSwapCalldata(*payload))
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(26062026)), key)
	if err != nil {
		t.Fatalf("sign swap: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	server := &OpsServer{peer: peer, evmChainID: 26062026, evmNetworkID: "26062026"}
	result, rpcErr := server.handleRPC(JSONRPCRequest{JSONRPC: "2.0", Method: "eth_sendRawTransaction", Params: []json.RawMessage{json.RawMessage(`"` + "0x" + common.Bytes2Hex(raw) + `"`)}})
	if rpcErr != nil {
		t.Fatalf("rpc error: %+v", rpcErr)
	}
	if result != strings.ToLower(signed.Hash().Hex()) {
		t.Fatalf("wrong hash: %v", result)
	}
	if peer.MempoolSize() != 1 {
		t.Fatalf("expected swap in mempool")
	}
}

func TestSwapQuoteEndpoint(t *testing.T) {
	peer, miner, _, _ := makeAMMTestPeer(t)
	poolID := account.PoolID(account.AssetPKN, account.AssetWPKN)
	applyAMMBlock(t, peer, miner,
		account.NewAssetCreditTransaction("credit-miner-wpkn", miner, account.AssetWPKN, miner.SafeEncode(), 10_000),
		account.NewAMMCreatePoolTransaction("create-pool", miner, account.AssetPKN, account.AssetWPKN),
		account.NewAMMAddLiquidityTransaction("add-liquidity", miner, poolID, 5_000, 5_000),
	)
	server := &OpsServer{peer: peer, evmChainID: 26062026, evmNetworkID: "26062026"}
	req := httptest.NewRequest(http.MethodGet, "/chain/swap/quote?pool="+poolID+"&assetIn=PKN&amountIn=100", nil)
	rec := httptest.NewRecorder()
	server.swapQuote(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("wrong status: %d %s", rec.Code, rec.Body.String())
	}
}

func signedSwapTxWithKey(t *testing.T, key *ecdsa.PrivateKey, nonce uint64, poolID string, assetIn string, assetOut string, amountIn int, minOut int) *account.SignedTransaction {
	t.Helper()
	payload := &account.AMMPayload{Action: "amm_swap", PoolID: poolID, AssetIn: assetIn, AssetOut: assetOut, AmountIn: amountIn, MinAmountOut: minOut}
	raw := mustSignedSwapRaw(t, key, nonce, payload)
	tx, err := account.NewEVMTransaction(raw, 26062026)
	if err != nil {
		t.Fatalf("decode raw swap: %v", err)
	}
	return tx
}

func mustSignedSwapRaw(t *testing.T, key *ecdsa.PrivateKey, nonce uint64, payload *account.AMMPayload) string {
	t.Helper()
	rawTx := types.NewTransaction(nonce, common.HexToAddress(account.PokoinSwapRouterAddress), big.NewInt(0), 120_000, big.NewInt(1_000_000_000), account.EncodePokoinSwapCalldata(*payload))
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(big.NewInt(26062026)), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return "0x" + common.Bytes2Hex(raw)
}
