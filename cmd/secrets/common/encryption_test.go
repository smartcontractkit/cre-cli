package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

const (
	testPassphrase  = "test-passphrase-for-ci"
	testExpectedHex = "521af99325c07c9bd0d224c5bf3ca25666c68b5fbb7fa7884019b4f60a8e6eb5"
)

func TestDeriveEncryptionKey_CrossLanguageVector(t *testing.T) {
	key, err := DeriveEncryptionKey(testPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(key)
	if got != testExpectedHex {
		t.Fatalf("HKDF vector mismatch:\n  got:  %s\n  want: %s", got, testExpectedHex)
	}
}

func TestAESGCMDecrypt_RoundTrip(t *testing.T) {
	key, err := DeriveEncryptionKey("round-trip-test")
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("hello confidential http")

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)

	got, err := AESGCMDecrypt(sealed, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestAESGCMDecrypt_TooShort(t *testing.T) {
	key, _ := DeriveEncryptionKey("any")
	_, err := AESGCMDecrypt(make([]byte, 10), key)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}
