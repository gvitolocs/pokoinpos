package dissycrypto

import "testing"

func TestKeySize(t *testing.T) {
	n, e, _, _ := KeyGen(2026)
	if n.BitLen() != 2026 {
		t.Errorf("n should be 2026 bits (is %d)", n.BitLen())
	}
	if e.Int64() != 3 {
		t.Errorf("e should be 3 (is %d)", e.Int64())
	}
}

func TestEncryptDecrypt(t *testing.T) {
	n, e, d, _ := KeyGen(1024)

	m := "A very secret message with answer 42."
	c := Encrypt(StringToBigInt(m), e, n)
	if c.String() == m {
		t.Errorf("Cipher text is equal to message!")
	}
	m_dec := BigIntToString(Decrypt(c, d, n))
	if m_dec != m {
		t.Errorf("Decrypted message does not equal original (decrypted is '%s')", m_dec)
	}
}
