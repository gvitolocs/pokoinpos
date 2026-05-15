package account

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"peer/dissycrypto"
)

type Account struct {
	n *big.Int
	e *big.Int
	d *big.Int
}

type AccountKeyMaterial struct {
	N string `json:"n"`
	E string `json:"e"`
	D string `json:"d"`
}

func NewAccount() (*Account, error) {
	n, e, d, err := dissycrypto.KeyGen(dissycrypto.WALLET_KEY_SIZE * 8)
	if err != nil {
		return nil, err
	}
	a := new(Account)
	a.n = n
	a.e = e
	a.d = d
	return a, nil
}

func (a *Account) SafeEncode() string {
	return encode(append(a.n.Bytes(), a.e.Bytes()...))
}

func PublicKeyFromAccountName(accountName string) ([]byte, error) {
	return decode(accountName)
}

func (a *Account) KeyMaterial() AccountKeyMaterial {
	return AccountKeyMaterial{
		N: base64.StdEncoding.EncodeToString(a.n.Bytes()),
		E: base64.StdEncoding.EncodeToString(a.e.Bytes()),
		D: base64.StdEncoding.EncodeToString(a.d.Bytes()),
	}
}

func AccountFromKeyMaterial(material AccountKeyMaterial) (*Account, error) {
	nBytes, err := base64.StdEncoding.DecodeString(material.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.StdEncoding.DecodeString(material.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	dBytes, err := base64.StdEncoding.DecodeString(material.D)
	if err != nil {
		return nil, fmt.Errorf("decode d: %w", err)
	}
	a := &Account{
		n: new(big.Int).SetBytes(nBytes),
		e: new(big.Int).SetBytes(eBytes),
		d: new(big.Int).SetBytes(dBytes),
	}
	return a, nil
}
