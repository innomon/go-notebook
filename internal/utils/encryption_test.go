package utils

import (
	"os"
	"testing"
)

func TestEncryptionDecryption(t *testing.T) {
	os.Setenv("OPEN_NOTEBOOK_ENCRYPTION_KEY", "test-secret-key-123456")
	defer os.Unsetenv("OPEN_NOTEBOOK_ENCRYPTION_KEY")

	plainText := "my-secret-api-key-value"
	cipherText, err := EncryptValue(plainText)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if !LooksLikeFernetToken(cipherText) {
		t.Fatalf("does not look like fernet token: %s", cipherText)
	}

	decrypted, err := DecryptValue(cipherText)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != plainText {
		t.Errorf("expected %q, got %q", plainText, decrypted)
	}
}

func TestLegacyPlainTextFallback(t *testing.T) {
	os.Setenv("OPEN_NOTEBOOK_ENCRYPTION_KEY", "test-secret-key-123456")
	defer os.Unsetenv("OPEN_NOTEBOOK_ENCRYPTION_KEY")

	plainText := "legacy-unencrypted-value"
	decrypted, err := DecryptValue(plainText)
	if err != nil {
		t.Fatalf("decryption of plain text failed: %v", err)
	}

	if decrypted != plainText {
		t.Errorf("expected %q, got %q", plainText, decrypted)
	}
}
