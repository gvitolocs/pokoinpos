package dissycrypto

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	TEST_WALLET_KEY_FILENAME := "./testfiles/walletkey-test1.sk"
	password := "verygoodpassword(TM)"
	pk := Generate(TEST_WALLET_KEY_FILENAME, password)
	pkBytes := []byte(pk)

	passwordHash, _ := hashPassword(password)
	sk, _ := DecryptFromFile(passwordHash[:], TEST_WALLET_KEY_FILENAME)
	if !compareSlices(pkBytes[:WALLET_KEY_SIZE], sk[:WALLET_KEY_SIZE]) {
		t.Errorf("sk and pk do not share n!")
	}
}

func TestGenerateAndSign(t *testing.T) {
	TEST_WALLET_KEY_FILENAME := "./testfiles/walletkey-test2.sk"
	CORRECT_PASSWORD := "verygoodpassword(TM)"
	pk := Generate(TEST_WALLET_KEY_FILENAME, CORRECT_PASSWORD)

	msg := []byte("An authentic message.")
	wrongSignature, _ := Sign(TEST_WALLET_KEY_FILENAME, "wrong password", msg)
	correctSignature, _ := Sign(TEST_WALLET_KEY_FILENAME, CORRECT_PASSWORD, msg)
	if VerifySignature(msg, wrongSignature, pk) {
		t.Errorf("Wrong signature passed test!")
	}
	if !VerifySignature(msg, correctSignature, pk) {
		t.Errorf("Correct signature failed test!")
	}
	if VerifySignature([]byte("A fake message"), correctSignature, pk) {
		t.Error("A fake message passed signature verification.")
	}
}

func compareSlices(v1, v2 []byte) bool {
	if len(v1) != len(v2) {
		return false
	}
	for idx := range len(v1) {
		if v1[idx] != v2[idx] {
			return false
		}
	}
	return true
}

func BenchmarkPasswordHashSpeed(b *testing.B) {
	for b.Loop() {
		hashPassword("verygoodpassword(TM)")
	}
}

/* Benchmark results:
cpu: 11th Gen Intel(R) Core(TM) i7-1165G7 @ 2.80GHz
BenchmarkPasswordHashSpeed-8   	       7	 161750529 ns/op	    1393 B/op	      12 allocs/op
*/
