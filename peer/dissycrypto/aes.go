package dissycrypto

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"io"
	"os"
)

func EncryptToFile(key []byte, plaintext []byte, filename string) error {
	// Create block cipher with key.
	block, err := aes.NewCipher(key) // Key must be 16, 24, or 32 bytes to use AES-128, AES-192, or AES-256, respectively.
	if err != nil {
		return err
	}

	// Make initialization vector and fill it with random bits.
	iv := make([]byte, block.BlockSize())
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return err
	}

	c := make([]byte, len(plaintext))
	ctr := cipher.NewCTR(block, iv)
	// Encipher plaintext with counter.
	ctr.XORKeyStream(c, plaintext)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	size := make([]byte, 2)
	binary.BigEndian.PutUint16(size, uint16(block.BlockSize()))
	// Write to file. First write size of blocks, then IV, then ciphertext.
	file.Write(size)
	file.Write(iv)
	file.Write(c)
	return nil
}

func DecryptFromFile(key []byte, filename string) ([]byte, error) {
	// Try to open the file.
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// Prepare reading.
	reader := bufio.NewReader(file)
	// Read size of blocks.
	size_arr := make([]byte, 2)
	_, err = io.ReadFull(reader, size_arr)
	if err != nil { // Trying to read wrong format.
		return nil, err
	}
	size := binary.BigEndian.Uint16(size_arr)
	// Read initializaion vector.
	iv := make([]byte, size)
	_, err = io.ReadFull(reader, iv)
	if err != nil { // Trying to read wrong format.
		return nil, err
	}
	// Read cipher text.
	c, err := io.ReadAll(reader)
	if err != nil { // Trying to read wrong format.
		return nil, err
	}
	// Create block counter and decrypt cipher text.
	block, err := aes.NewCipher(key)
	if err != nil { // Trying to read wrong format.
		return nil, err
	}
	ctr := cipher.NewCTR(block, iv)
	m := make([]byte, len(c))
	ctr.XORKeyStream(m, c)
	return m, nil
}

//func main() {
//	file := "./test.cifer"
//	n, _, _, _ := KeyGen(32 * 8) // 32 byte key.
//	EncryptToFile(n.Bytes(), []byte("Top secret: Ice cream is great..."), file)
//	DecryptFromFile(n.Bytes(), file)
//}
