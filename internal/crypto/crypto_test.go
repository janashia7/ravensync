package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}
	if len(salt) != SaltSize {
		t.Fatalf("salt length = %d, want %d", len(salt), SaltSize)
	}

	salt2, _ := GenerateSalt()
	if bytes.Equal(salt, salt2) {
		t.Fatal("two salts should not be equal")
	}
}

func TestDeriveKey(t *testing.T) {
	salt, _ := GenerateSalt()
	key, err := DeriveKey([]byte("password"), salt)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if len(key) != KeySize {
		t.Fatalf("key length = %d, want %d", len(key), KeySize)
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt, _ := GenerateSalt()
	k1, _ := DeriveKey([]byte("same-password"), salt)
	k2, _ := DeriveKey([]byte("same-password"), salt)
	if !bytes.Equal(k1, k2) {
		t.Fatal("same password + salt should produce same key")
	}
}

func TestDeriveKeyDifferentPasswords(t *testing.T) {
	salt, _ := GenerateSalt()
	k1, _ := DeriveKey([]byte("password-a"), salt)
	k2, _ := DeriveKey([]byte("password-b"), salt)
	if bytes.Equal(k1, k2) {
		t.Fatal("different passwords should produce different keys")
	}
}

func TestDeriveKeyEmptySalt(t *testing.T) {
	_, err := DeriveKey([]byte("password"), nil)
	if err == nil {
		t.Fatal("expected error for empty salt")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, KeySize)
	copy(key, []byte("test-key-32-bytes-long-padding!!"))

	plaintext := []byte("hello ravensync")
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	key := make([]byte, KeySize)
	copy(key, []byte("test-key-32-bytes-long-padding!!"))

	c1, _ := Encrypt([]byte("same data"), key)
	c2, _ := Encrypt([]byte("same data"), key)
	if bytes.Equal(c1, c2) {
		t.Fatal("encrypting same data twice should produce different ciphertexts (unique nonces)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, KeySize)
	copy(key1, []byte("key-one-32-bytes-long-padding!!!"))
	key2 := make([]byte, KeySize)
	copy(key2, []byte("key-two-32-bytes-long-padding!!!"))

	ciphertext, _ := Encrypt([]byte("secret"), key1)
	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := make([]byte, KeySize)
	copy(key, []byte("test-key-32-bytes-long-padding!!"))
	_, err := Decrypt([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestEncryptEmpty(t *testing.T) {
	key := make([]byte, KeySize)
	copy(key, []byte("test-key-32-bytes-long-padding!!"))

	ct, err := Encrypt([]byte{}, key)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	pt, err := Decrypt(ct, key)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(pt) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(pt))
	}
}
