package utils

import (
	"os"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Skip("ENCRYPTION_KEY not set - skipping encryption test")
	}

	original := "test-access-token-12345"

	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted == original {
		t.Fatal("Encrypted text should be different from original")
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Decrypted text doesn't match original: got %s, want %s", decrypted, original)
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Skip("ENCRYPTION_KEY not set - skipping encryption test")
	}

	original := "same-text"

	encrypted1, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	encrypted2, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if encrypted1 == encrypted2 {
		t.Fatal("Same text should produce different ciphertext (due to random IV)")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Skip("ENCRYPTION_KEY not set - skipping encryption test")
	}

	_, err := Decrypt("invalid-hex-data")
	if err == nil {
		t.Fatal("Should fail with invalid hex data")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Skip("ENCRYPTION_KEY not set - skipping encryption test")
	}

	original := ""

	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt empty string: %v", err)
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt empty string: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Decrypted empty string doesn't match: got %s", decrypted)
	}
}

func TestEncryptLongString(t *testing.T) {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Skip("ENCRYPTION_KEY not set - skipping encryption test")
	}

	original := "this-is-a-very-long-string-that-should-still-encrypt-correctly-and-decrypt-properly-without-any-issues"

	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Failed to encrypt long string: %v", err)
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt long string: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Decrypted long string doesn't match")
	}
}
