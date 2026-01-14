package services

import (
	"strings"
	"testing"
)

func TestNewEncryptionService(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		expectErr bool
	}{
		{
			name:      "Valid key",
			key:       "my-secret-encryption-key-12345",
			expectErr: false,
		},
		{
			name:      "Empty key",
			key:       "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewEncryptionService(tt.key)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if svc == nil {
					t.Error("Expected service but got nil")
				}
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	svc, err := NewEncryptionService("my-secret-encryption-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	testCases := []string{
		"Hello, World!",
		"WiFi Password: secret123",
		"Long text with special characters: !@#$%^&*()_+-=[]{}|;':\",./<>?",
		"Unicode: 你好世界 🎉",
		"",
	}

	for _, plaintext := range testCases {
		t.Run(plaintext, func(t *testing.T) {
			encrypted, err := svc.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if plaintext != "" && encrypted == plaintext {
				t.Error("Encrypted text should differ from plaintext")
			}

			decrypted, err := svc.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != plaintext {
				t.Errorf("Decrypted text doesn't match: got '%s', want '%s'", decrypted, plaintext)
			}
		})
	}
}

func TestEncryptDifferentOutputs(t *testing.T) {
	svc, err := NewEncryptionService("my-secret-encryption-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	plaintext := "Same text"
	encrypted1, _ := svc.Encrypt(plaintext)
	encrypted2, _ := svc.Encrypt(plaintext)

	// Due to random nonce, encrypting the same text should produce different outputs
	if encrypted1 == encrypted2 {
		t.Error("Encrypting same text twice should produce different outputs due to random nonce")
	}

	// But both should decrypt to the same plaintext
	decrypted1, _ := svc.Decrypt(encrypted1)
	decrypted2, _ := svc.Decrypt(encrypted2)

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both encrypted texts should decrypt to original plaintext")
	}
}

func TestDecryptInvalidInput(t *testing.T) {
	svc, err := NewEncryptionService("my-secret-encryption-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	invalidInputs := []string{
		"not-valid-base64!!!",
		"YWJj", // Valid base64 but too short for AES-GCM
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			_, err := svc.Decrypt(input)
			if err == nil {
				t.Error("Expected error for invalid input")
			}
		})
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	svc1, _ := NewEncryptionService("encryption-key-one-12345678901")
	svc2, _ := NewEncryptionService("encryption-key-two-12345678901")

	plaintext := "Secret message"
	encrypted, _ := svc1.Encrypt(plaintext)

	_, err := svc2.Decrypt(encrypted)
	if err == nil {
		t.Error("Decrypting with wrong key should fail")
	}
}

func TestHash(t *testing.T) {
	svc, err := NewEncryptionService("my-secret-encryption-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	tests := []struct {
		input string
	}{
		{"Hello, World!"},
		{"Test data"},
		{""},
		{"Unicode: 你好世界"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hash := svc.Hash(tt.input)

			// SHA-256 produces 64 character hex string
			if len(hash) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(hash))
			}

			// Hash should be consistent
			hash2 := svc.Hash(tt.input)
			if hash != hash2 {
				t.Error("Hash should be consistent for same input")
			}

			// Hash should be hex characters only
			for _, c := range hash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("Hash contains non-hex character: %c", c)
				}
			}
		})
	}
}

func TestHashDifferentInputs(t *testing.T) {
	svc, _ := NewEncryptionService("my-secret-encryption-key-12345")

	hash1 := svc.Hash("input1")
	hash2 := svc.Hash("input2")

	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestGenerateRandomKey(t *testing.T) {
	svc, err := NewEncryptionService("my-secret-encryption-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	lengths := []int{16, 32, 64}

	for _, length := range lengths {
		t.Run(string(rune(length)), func(t *testing.T) {
			key, err := svc.GenerateRandomKey(length)
			if err != nil {
				t.Fatalf("GenerateRandomKey failed: %v", err)
			}

			// Hex encoding doubles the length
			expectedLen := length * 2
			if len(key) != expectedLen {
				t.Errorf("Expected key length %d, got %d", expectedLen, len(key))
			}

			// Should be hex characters only
			for _, c := range key {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("Key contains non-hex character: %c", c)
				}
			}
		})
	}
}

func TestGenerateRandomKeyUnique(t *testing.T) {
	svc, _ := NewEncryptionService("my-secret-encryption-key-12345")

	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, _ := svc.GenerateRandomKey(32)
		if keys[key] {
			t.Error("Generated duplicate key")
		}
		keys[key] = true
	}
}

func TestEncryptionServiceWithDifferentKeyLengths(t *testing.T) {
	// Test that various key lengths work (they get hashed to 32 bytes)
	keyLengths := []int{8, 16, 32, 64, 128}

	for _, length := range keyLengths {
		key := strings.Repeat("a", length)
		t.Run(key, func(t *testing.T) {
			svc, err := NewEncryptionService(key)
			if err != nil {
				t.Fatalf("Failed to create service with key length %d: %v", length, err)
			}

			plaintext := "Test message"
			encrypted, err := svc.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			decrypted, err := svc.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != plaintext {
				t.Errorf("Round-trip failed: got '%s', want '%s'", decrypted, plaintext)
			}
		})
	}
}
