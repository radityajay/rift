package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// Encrypt encrypts plaintext using ChaCha20-Poly1305 with the given session key.
// Returns nonce || ciphertext.
func Encrypt(sessionKey [KeySize]byte, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(sessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext (nonce || encrypted) using ChaCha20-Poly1305.
func Decrypt(sessionKey [KeySize]byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(sessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
