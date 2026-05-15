package dissycrypto

import "math/big"

func StringToBigInt(m string) *big.Int {
	return BytesToBigInt([]byte(m))
}

func BytesToBigInt(b []byte) *big.Int {
	result := new(big.Int)
	result.SetBytes(b)
	return result
}

func BigIntToString(b *big.Int) string {
	return string(b.Bytes())
}
