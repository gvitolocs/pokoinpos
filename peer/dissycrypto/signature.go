package dissycrypto

import (
	"crypto/sha256"
	"math/big"
)

func RSASign(message []byte, d, n *big.Int) *big.Int {
	hash := applyHash(message)
	return Decrypt(BytesToBigInt(hash[:]), d, n)
}

func RSAVerify(message []byte, signature, e, n *big.Int) bool {
	signatureHash := Encrypt(signature, e, n)
	messageHash := applyHash(message)
	return signatureHash.Cmp(BytesToBigInt(messageHash[:])) == 0 // Check if signHash == msgHash
}

func applyHash(message []byte) [32]byte {
	return sha256.Sum256(message)
}
