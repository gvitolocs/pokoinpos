package account

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestEVMTransactionVerifiesAndAdvancesNonce(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate evm key: %v", err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x1000000000000000000000000000000000000001")
	chainID := big.NewInt(26062026)
	rawTx := types.NewTransaction(0, to, new(big.Int).Mul(big.NewInt(10), EVMUnit), 21_000, big.NewInt(0), nil)
	signed, err := types.SignTx(rawTx, types.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}

	tx, err := NewEVMTransaction("0x"+common.Bytes2Hex(raw), chainID.Int64())
	if err != nil {
		t.Fatalf("decode evm transaction: %v", err)
	}
	if tx.From != strings.ToLower(from.Hex()) {
		t.Fatalf("wrong sender: got %s want %s", tx.From, strings.ToLower(from.Hex()))
	}
	if !tx.Verify(tx.From) {
		t.Fatalf("expected evm tx to verify")
	}

	ledger := MakeLedger()
	ledger.SetBalance(tx.From, 100)
	if !ledger.Transaction(tx) {
		t.Fatalf("expected first nonce to apply")
	}
	if got := ledger.GetNonce(tx.From); got != 1 {
		t.Fatalf("wrong nonce after apply: got %d want 1", got)
	}
	if ledger.Transaction(tx) {
		t.Fatalf("expected replay with same nonce to fail")
	}
}
