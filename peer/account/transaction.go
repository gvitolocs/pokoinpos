package account

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"peer/dissycrypto"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type SignedTransaction struct {
	ID        string
	From      string
	To        string
	Amount    int
	Signature string
	Kind      string
	Nonce     uint64
	Raw       string
	ChainID   int64
}

var EVMUnit = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// Perform transactions. Checks whether transaction is authentic before applying.
func (l *Ledger) Transaction(t *SignedTransaction) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	if t.IsMint() {
		if t.Amount < 1 || l.Accounts[t.From] <= 0 || !t.Verify(t.From) {
			return false
		}
		l.TxHistory[t.ID] = *t
		l.Accounts[t.To] += t.Amount
		return true
	}
	// Native RSA transactions must transfer a positive amount. EVM zero-value
	// transactions are allowed so wallets can cancel/replace pending nonces.
	if t.Amount < 0 || (!t.IsEVM() && t.Amount < 1) {
		return false
	}
	if !t.Verify(t.From) {
		return false
	}
	if t.IsEVM() {
		if l.Nonces[t.From] != t.Nonce {
			return false
		}
	}
	// Exercise 16.2: reject overdrafts (no negative balances).
	if l.Accounts[t.From] < t.Amount {
		return false
	}

	l.TxHistory[t.ID] = *t

	l.Accounts[t.From] -= t.Amount
	l.Accounts[t.To] += t.Amount
	if t.IsEVM() {
		l.Nonces[t.From] = t.Nonce + 1
	}
	return true
}

func (l *Ledger) HasRecordedTransaction(tx *SignedTransaction) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	_, exists := l.TxHistory[tx.ID]
	return exists
}

func NewSignedTransaction(ID string, From *Account, To string, Amount int) *SignedTransaction {
	s := new(SignedTransaction)
	s.ID = ID
	s.From = From.SafeEncode()
	s.To = To
	s.Amount = Amount
	s.signTransaction(From.d, From.n)
	return s
}

func NewMintTransaction(ID string, From *Account, To string, Amount int) *SignedTransaction {
	s := NewSignedTransaction(ID, From, To, Amount)
	s.Kind = "mint"
	s.signTransaction(From.d, From.n)
	return s
}

func NewEVMTransaction(raw string, chainID int64) (*SignedTransaction, error) {
	normalized := strings.TrimPrefix(strings.TrimSpace(raw), "0x")
	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex("0x" + normalized)); err != nil {
		return nil, err
	}
	signer := types.LatestSignerForChainID(big.NewInt(chainID))
	from, err := types.Sender(signer, &tx)
	if err != nil {
		return nil, err
	}
	to := ""
	if tx.To() != nil {
		to = strings.ToLower(tx.To().Hex())
	}
	value := tx.Value()
	if value == nil || value.Sign() < 0 {
		return nil, fmt.Errorf("evm value must be non-negative")
	}
	amount, remainder := new(big.Int).QuoRem(value, EVMUnit, new(big.Int))
	if remainder.Sign() != 0 {
		return nil, fmt.Errorf("evm value must be a whole PKN amount")
	}
	if !amount.IsInt64() || amount.Int64() > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("evm value must fit native amount")
	}
	return &SignedTransaction{
		ID:        strings.ToLower(tx.Hash().Hex()),
		From:      strings.ToLower(from.Hex()),
		To:        to,
		Amount:    int(amount.Int64()),
		Signature: "evm:" + strings.TrimPrefix(strings.TrimSpace(raw), "0x"),
		Kind:      "evm",
		Nonce:     tx.Nonce(),
		Raw:       "0x" + normalized,
		ChainID:   chainID,
	}, nil
}

func (s *SignedTransaction) IsEVM() bool {
	return s.Kind == "evm" || strings.HasPrefix(s.Signature, "evm:")
}

func (s *SignedTransaction) IsMint() bool {
	return s.Kind == "mint"
}

func (s *SignedTransaction) marshalContentForSignature() ([]byte, error) {
	return json.Marshal([]string{s.ID, s.From, s.To, strconv.Itoa(s.Amount), s.Kind})
}

func (s *SignedTransaction) signTransaction(d, n *big.Int) {
	data, err := s.marshalContentForSignature()
	if err != nil {
		return
	}
	s.Signature = encode(dissycrypto.RSASign(data, d, n).Bytes())
}

func encode(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

func decode(content string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(content)
}

func (s *SignedTransaction) Verify(accountName string) bool {
	if s.IsEVM() {
		tx, err := s.decodeEVM()
		if err != nil {
			return false
		}
		signer := types.LatestSignerForChainID(big.NewInt(s.ChainID))
		from, err := types.Sender(signer, tx)
		if err != nil {
			return false
		}
		if !strings.EqualFold(from.Hex(), accountName) {
			return false
		}
		if !strings.EqualFold(tx.Hash().Hex(), s.ID) {
			return false
		}
		if tx.To() == nil || !strings.EqualFold(tx.To().Hex(), s.To) {
			return false
		}
		expectedValue := new(big.Int).Mul(big.NewInt(int64(s.Amount)), EVMUnit)
		if tx.Value() == nil || tx.Value().Cmp(expectedValue) != 0 {
			return false
		}
		return tx.Nonce() == s.Nonce
	}
	data, err := s.marshalContentForSignature()
	if err != nil {
		return false
	}
	signature, err := decode(s.Signature)
	if err != nil {
		return false
	}
	pk, err := PublicKeyFromAccountName(accountName)
	if err != nil {
		return false
	}
	return dissycrypto.VerifySignature(data, signature, string(pk))
}

func (s *SignedTransaction) decodeEVM() (*types.Transaction, error) {
	raw := s.Raw
	if raw == "" {
		raw = strings.TrimPrefix(s.Signature, "evm:")
	}
	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex("0x" + strings.TrimPrefix(raw, "0x"))); err != nil {
		return nil, err
	}
	return &tx, nil
}
