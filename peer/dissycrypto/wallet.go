package dissycrypto

import (
	"crypto/pbkdf2"
	"crypto/sha512"
	"fmt"
	"math/big"
	"os"
)

const WALLET_KEY_SIZE = 2048 / 8 // Key size in bytes.
const HASH_SIZE = 32
const PASSWORD_HASH_ITERATION_COUNT = 300000 // The number of iterations to run when hashing password.

/*
Create an  RSA sk/pk pair and store the sk in a file (given by filename)
encrypted under the password.
Returns the public key.
*/
func Generate(filename string, password string) string {
	// Create RSA key.
	n, e, d, _ := KeyGen(WALLET_KEY_SIZE * 8)
	sk := append(n.Bytes(), d.Bytes()...)
	pk := append(n.Bytes(), e.Bytes()...)
	// Hash the password.
	passwordHash, err := hashPassword(password)
	if err != nil {
		return ""
	}
	// Encrypt and save the secret key.
	EncryptToFile(passwordHash, sk, filename)
	return string(pk)
}

/*
Sign a message (msg) using the secret key stored in the file given by filename.
Returns the signature.
*/
func Sign(filename string, password string, msg []byte) ([]byte, error) {
	// Check file exists.
	if _, err := os.Stat(filename); err != nil {
		return nil, err
	}
	// Get hold of the secret key stored in the file.
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	sk, err := DecryptFromFile(passwordHash, filename)
	if err != nil {
		return nil, err
	}
	var n, d big.Int
	n.SetBytes(sk[:WALLET_KEY_SIZE])
	d.SetBytes(sk[WALLET_KEY_SIZE:])
	// Sign the message and return the signature.
	signature := RSASign(msg, &d, &n)
	return signature.Bytes(), nil
}

// Wrapper around RSAVerify to handle public key and signature in string and byte format, respectively.
func VerifySignature(msg, signature []byte, pk string) bool {
	pkBytes := []byte(pk)
	var n, e, s big.Int
	n.SetBytes(pkBytes[:WALLET_KEY_SIZE])
	e.SetBytes(pkBytes[WALLET_KEY_SIZE:])
	s.SetBytes(signature)
	return RSAVerify(msg, &s, &e, &n)
}

func hashPassword(password string) ([]byte, error) {
	salt := []byte("salt") // We do not use different salts in this exercise.
	return pbkdf2.Key(sha512.New, password, salt, PASSWORD_HASH_ITERATION_COUNT, HASH_SIZE)
}

func main() {
	password := "verygoodpassword(TM)"
	filename := "./testfiles/walletkey.sk"
	fmt.Printf("Starting Wallet Demo.\nPassword: %s\nStorage location of secret key: %s\n", password, filename)
	msg := []byte("An authentic message sent by DA1 Command HQ.")
	pk := Generate(filename, password)
	sign, err := Sign(filename, password, msg)
	if err != nil {
		fmt.Println(err)
		return
	}
	verify := VerifySignature(msg, sign, pk)

	fmt.Printf("Verification: %t", verify)
}
