package crypto

import (
	"bytes"
	"testing"
)

func TestKeypairGeneration(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	// Public key should not be all zeros
	var zero [KeySize]byte
	if kp.Public == zero {
		t.Fatal("public key is all zeros")
	}
	if kp.Private == zero {
		t.Fatal("private key is all zeros")
	}
}

func TestSharedSecret(t *testing.T) {
	alice, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("generate alice: %v", err)
	}
	bob, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("generate bob: %v", err)
	}

	secretA, err := SharedSecret(alice.Private, bob.Public)
	if err != nil {
		t.Fatalf("shared secret alice: %v", err)
	}
	secretB, err := SharedSecret(bob.Private, alice.Public)
	if err != nil {
		t.Fatalf("shared secret bob: %v", err)
	}

	if secretA != secretB {
		t.Fatal("shared secrets do not match")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	alice, _ := GenerateKeypair()
	bob, _ := GenerateKeypair()
	sessionKey, _ := SharedSecret(alice.Private, bob.Public)

	plaintext := []byte(`{"event":"payment.success","amount":50000}`)

	ciphertext, err := Encrypt(sessionKey, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	decrypted, err := Decrypt(sessionKey, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted does not match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	alice, _ := GenerateKeypair()
	bob, _ := GenerateKeypair()
	eve, _ := GenerateKeypair()

	sessionKey, _ := SharedSecret(alice.Private, bob.Public)
	wrongKey, _ := SharedSecret(eve.Private, bob.Public)

	ciphertext, _ := Encrypt(sessionKey, []byte("secret"))
	_, err := Decrypt(wrongKey, ciphertext)
	if err == nil {
		t.Fatal("expected decrypt error with wrong key")
	}
}

func TestEncryptDecryptEmptyPayload(t *testing.T) {
	alice, _ := GenerateKeypair()
	bob, _ := GenerateKeypair()
	sessionKey, _ := SharedSecret(alice.Private, bob.Public)

	ciphertext, err := Encrypt(sessionKey, []byte{})
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(sessionKey, ciphertext)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(decrypted))
	}
}
