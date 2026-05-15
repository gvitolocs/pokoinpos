package dissycrypto

import (
	"crypto/rand"
	"math/big"
)

func KeyGen(k int) (n, e, d *big.Int, err error) {
	// p and q are k/2 bits long which makes n being k/2 + k/2 = k bits long.
	for {
		p, err := rand.Prime(rand.Reader, k/2)
		if err != nil {
			return nil, nil, nil, err
		}
		q, err := rand.Prime(rand.Reader, k/2)
		if err != nil {
			return nil, nil, nil, err
		}
		e = big.NewInt(3)
		n = new(big.Int)
		n.Mul(p, q)
		if acceptPrimes(p, q, e) {
			d = new(big.Int)
			one := big.NewInt(1)
			d.ModInverse(e, d.Mul(p.Sub(p, one), q.Sub(q, one)))
			break
		}
	}
	return n, e, d, nil
}

func acceptPrimes(p, q, e *big.Int) bool {
	one := big.NewInt(1)
	gcd1 := new(big.Int)
	gcd2 := new(big.Int)
	gcd1.GCD(nil, nil, gcd1.Sub(p, one), e)
	gcd2.GCD(nil, nil, gcd2.Sub(q, one), e)
	// Cmp return 0 if values are equal...
	return (gcd1.Cmp(one) == 0) && (gcd2.Cmp(one) == 0)
}

func Encrypt(m, e, n *big.Int) *big.Int {
	c := new(big.Int)
	c.Exp(m, e, n) // Calculates m**e mod |n|
	return c
}

func Decrypt(c, d, n *big.Int) *big.Int {
	m := new(big.Int)
	m.Exp(c, d, n)
	return m
}
