package dissycrypto

import (
	"io"
	"os"
	"testing"
)

func TestAES(t *testing.T) {
	// Create key for AES encryption.
	key, _, _, _ := KeyGen(32 * 8) // 32 byte key.
	filename := "./testfiles/test1.txt"
	m := "TOP SECRET: The answer is 42."
	EncryptToFile(key.Bytes(), []byte(m), filename)
	m_dec, _ := DecryptFromFile(key.Bytes(), filename)
	if m != string(m_dec) {
		t.Errorf("Deciphered message not equal to original. Deciphered message was: '%s'", m_dec)
	}
}

/*
func TestAES_RSA(t *testing.T) {
	msgFile := "./test_aes_rsa.ciphertext"
	keyFile := "test_rsa-aes.key"
	msg := "TOP SECRET: Operation DA1\nSteps shall be executed exactly as stated in the following text.\nStep 1) Write assignment.\n2) Have it approved.\n3) Bask in glory.\nDA1 HQ."
	// **** SENDER SIDE ****
	// Create RSA key.
	{
		n, e, d, _ := KeyGen(1024)

		// Encrypt a message.
		msgEnc := Encrypt(StringToBigInt(msg), e, n)
		// Send encrypted message.
		file, err := os.Create(msgFile)
		if err != nil {
			t.Errorf("Could not create file!")
		}
		file.Write(msgEnc.Bytes())
		file.Close()

		// Prepare to send secret key.
		nBytes := n.Bytes()
		nSize := make([]byte, 4)
		binary.BigEndian.PutUint32(nSize, uint32(len(nBytes)))
		dBytes := d.Bytes()
		dSize := make([]byte, 4)
		binary.BigEndian.PutUint32(dSize, uint32(len(dBytes)))
		m := append(nSize, nBytes...)
		m = append(m, dSize...)
		m = append(m, dBytes...)

		// Encrypt the secret key.
		key, _, _, _ := KeyGen(32 * 8)
		EncryptToFile(key.Bytes(), m, keyFile)
	}

	// **** RECEIVER SIDE ****
	{
		// Decrypt RSA secret key.
		mDec, _ := DecryptFromFile(key.Bytes(), keyFile)
		nSizeDec := binary.BigEndian.Uint32(mDec[:4])
		n := new(big.Int)
		n.SetBytes(mDec[4 : 4+nSizeDec])
		dSizeDec := binary.BigEndian.Uint32(mDec[4+nSizeDec : 4+nSizeDec+4])
		d := new(big.Int)
		d.SetBytes(mDec[4+nSizeDec+4 : 4+nSizeDec+4+dSizeDec])

		// Read encrypted message.
		file, err := os.Open(msgFile)
		if err != nil {
			t.Errorf("Failed to read file!")
		}
		msgDec, err := io.ReadAll(file)
		msgDecInt := new(big.Int)
		msgDecInt.SetBytes(msgDec)
		msgDecInt = Decrypt(msgDecInt, d, n)
	}

	t.Errorf("key %s", mDec)
}
*/

func TestHybrid(t *testing.T) {
	msgFile := "./testfiles/test2.txt"
	keyFile := "./testfiles/test2.sk"
	msg := "TOP SECRET: Operation Handin3\nSteps shall be executed exactly as stated in the following text.\nStep 1) Write assignment.\nStep 2) Await approval.\nStep 3) Bask in glory.\nDA1 Command Centre, HQ."
	// Create RSA key.
	n, e, d, _ := KeyGen(1024) // Technically generated at receiver, and (n, e) is sent to sender.
	// **** SENDER SIDE ****
	{
		// Encrypt a message to file using AES.
		key, _, _, _ := KeyGen(32 * 8)
		err := EncryptToFile(key.Bytes(), StringToBigInt(msg).Bytes(), msgFile)
		if err != nil {
			t.Errorf("Could not send file!")
		}

		// Send encrypted AES (secret) key using RSA public key.
		keyMsg := Encrypt(key, e, n)
		err = writeFile(keyMsg.Bytes(), keyFile)
		if err != nil {
			t.Errorf("Could not create file!")
		}
	}

	// **** RECEIVER SIDE ****
	{
		// Decrypt AES secret key.
		file, err := os.Open(keyFile)
		if err != nil {
			t.Errorf("Failed to read file!")
		}
		defer file.Close()
		keyEnc, err := io.ReadAll(file)
		key := Decrypt(BytesToBigInt(keyEnc), d, n)

		// Read encrypted message.
		msgDec, err := DecryptFromFile(key.Bytes(), msgFile)
		if string(msgDec) != msg {
			t.Errorf("Decrypted file not equal to original message!")
		}
	}

	// **** POOR BAD GUY TRYING TO USE ENCRYPTED SK ****
	{
		// Showing that one cannot use encrypted SK to generate msg.
		file, err := os.Open(keyFile)
		if err != nil {
			t.Errorf("Failed to read file!")
		}
		defer file.Close()
		key, _ := io.ReadAll(file)

		// Read encrypted message.
		msgDec, err := DecryptFromFile(key, msgFile) // Should get err.
		if string(msgDec) == msg {
			t.Errorf("Bad guy used SK to read secret message!")
		}
	}
}

func writeFile(content []byte, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(content)
	return nil
}
