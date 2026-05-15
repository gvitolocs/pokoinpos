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

// Perform transactions. Checks whether transaction is authentic before applying.
func (l *Ledger) Transaction(t *SignedTransaction) bool {
	l.lock.Lock()
	defer l.lock.Unlock()
	// Exercise 16.2: amount must be positive.
	if t.Amount < 1 {
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
	// Exercise 16.2: receiver gets 1 PK less; this difference is the fee.
	l.Accounts[t.To] += t.Amount - 1
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
	if value == nil || !value.IsInt64() {
		return nil, fmt.Errorf("evm value must fit signed 64-bit integer")
	}
	if value.Sign() <= 0 || value.Int64() > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("evm value must be positive and fit int")
	}
	return &SignedTransaction{
		ID:        strings.ToLower(tx.Hash().Hex()),
		From:      strings.ToLower(from.Hex()),
		To:        to,
		Amount:    int(value.Int64()),
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

func (s *SignedTransaction) marshalContentForSignature() ([]byte, error) {
	return json.Marshal([]string{s.ID, s.From, s.To, strconv.Itoa(s.Amount)})
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
		if tx.Value() == nil || !tx.Value().IsInt64() || int(tx.Value().Int64()) != s.Amount {
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
