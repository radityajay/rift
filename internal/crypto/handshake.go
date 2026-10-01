package crypto

import (
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"crypto/sha256"
)

const KeySize = 32

// Keypair holds an X25519 private/public key pair.
type Keypair struct {
	Private [KeySize]byte
	Public  [KeySize]byte
}

// GenerateKeypair creates a new ephemeral X25519 keypair.
func GenerateKeypair() (*Keypair, error) {
	kp := &Keypair{}
	if _, err := rand.Read(kp.Private[:]); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}
	pub, err := curve25519.X25519(kp.Private[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}
	copy(kp.Public[:], pub)
	return kp, nil
}

// SharedSecret performs X25519 Diffie-Hellman and derives a session key via HKDF.
func SharedSecret(private [KeySize]byte, peerPublic [KeySize]byte) ([KeySize]byte, error) {
	var sessionKey [KeySize]byte

	shared, err := curve25519.X25519(private[:], peerPublic[:])
	if err != nil {
		return sessionKey, fmt.Errorf("x25519 exchange: %w", err)
	}

	// Derive session key using HKDF-SHA256
	hk := hkdf.New(sha256.New, shared, nil, []byte("rift-session-v1"))
	if _, err := io.ReadFull(hk, sessionKey[:]); err != nil {
		return sessionKey, fmt.Errorf("hkdf derive: %w", err)
	}

	return sessionKey, nil
}
