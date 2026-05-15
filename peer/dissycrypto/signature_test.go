package dissycrypto

import (
	"crypto/sha256"
	"math/big"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	msg := "A message to sign."
	n, e, d, _ := KeyGen(512) // key must be larger than hash (256 bits)
	sign := RSASign([]byte(msg), d, n)
	if !RSAVerify([]byte(msg), sign, e, n) {
		t.Errorf("Message not verified, but shoud have been!")
	}
	badMsg := "An aletered message with different content."
	if RSAVerify([]byte(badMsg), sign, e, n) {
		t.Errorf("Bad message was verified!")
	}
	if RSAVerify([]byte{0}, big.NewInt(0), e, n) { // Just showing that (m, sign) pairs that are always valid in normal RSA-signature are not valid here.
		t.Errorf("Bad message was verified!")
	}
	if RSAVerify([]byte{1}, big.NewInt(1), e, n) { // Just showing that (m, sign) pairs that are always valid in normal RSA-signature are not valid here.
		t.Errorf("Bad message was verified!")
	}
}

func BenchmarkHashSpeed(b *testing.B) {
	msg := make([]byte, 10_000) // 10 KB message
	for i := 0; i < b.N; i++ {
		sha256.Sum256(msg)
	}
}

/* Result (Exercise 6.15 numbers):
4200 ns to hash 10 KB. 10 KB = 10,000 bytes = 80,000 bits.
Throughput: 80,000 / (4,200 × 10^-9) ≈ 1.95 × 10^10 bits/s ≈ 19 Gbit/s
*/

func BenchmarkSignatureOnHash(b *testing.B) {
	hashSizedMsg := make([]byte, 32) // sign a hash-sized input (SHA-256 output)
	n, _, d, _ := KeyGen(2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RSASign(hashSizedMsg, d, n)
	}
}

/* Result (Exercise 6.15 numbers):
1,970,926 ns/op ≈ 1.97 ms per RSA signature (2000-bit key, on hash value).
*/
