package account

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"peer/dissycrypto"
	"strconv"
)

type SignedTransaction struct {
	ID        string
	From      string
	To        string
	Amount    int
	Signature string
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
	// Exercise 16.2: reject overdrafts (no negative balances).
	if l.Accounts[t.From] < t.Amount {
		return false
	}

	l.TxHistory[t.ID] = *t

	l.Accounts[t.From] -= t.Amount
	// Exercise 16.2: receiver gets 1 PK less; this difference is the fee.
	l.Accounts[t.To] += t.Amount - 1
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
