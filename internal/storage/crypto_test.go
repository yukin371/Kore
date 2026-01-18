package storage

import (
	"bytes"
	"testing"
)

func TestAESGCMEncryptor(t *testing.T) {
	// 测试不同大小的密钥
	testCases := []struct {
		name    string
		keySize int
	}{
		{"AES-128", 16},
		{"AES-192", 24},
		{"AES-256", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keySize)
			for i := range key {
				key[i] = byte(i)
			}

			encryptor, err := NewAESGCMEncryptor(key)
			if err != nil {
				t.Fatalf("Failed to create encryptor: %v", err)
			}

			// 测试加密解密
			plaintext := []byte("This is a secret message")
			ciphertext, err := encryptor.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Failed to encrypt: %v", err)
			}

			// 密文应该与明文不同
			if bytes.Equal(ciphertext, plaintext) {
				t.Error("Ciphertext should be different from plaintext")
			}

			// 解密
			decrypted, err := encryptor.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Failed to decrypt: %v", err)
			}

			// 解密后应该与原文相同
			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("Decrypted text doesn't match original: got %s, want %s", decrypted, plaintext)
			}
		})
	}
}

func TestAESGCMEncryptorInvalidKey(t *testing.T) {
	invalidKeys := [][]byte{
		[]byte("short"),       // 太短
		[]byte("waytoolongkey"), // 不在有效范围内
	}

	for _, key := range invalidKeys {
		_, err := NewAESGCMEncryptor(key)
		if err == nil {
			t.Error("Expected error for invalid key size, got nil")
		}
	}
}

func TestAESGCMEncryptorString(t *testing.T) {
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i)
	}

	encryptor, err := NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("Secret message for string encoding")

	// 加密为字符串
	encryptedStr, err := encryptor.EncryptToString(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt to string: %v", err)
	}

	if encryptedStr == "" {
		t.Error("Encrypted string should not be empty")
	}

	// 从字符串解密
	decrypted, err := encryptor.DecryptFromString(encryptedStr)
	if err != nil {
		t.Fatalf("Failed to decrypt from string: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted text doesn't match original: got %s, want %s", decrypted, plaintext)
	}
}

func TestAESGCMEncryptorEmptyInput(t *testing.T) {
	key := make([]byte, 32)
	encryptor, err := NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 测试空输入
	plaintext := []byte("")
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt empty input: %v", err)
	}

	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt empty input: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted empty text doesn't match original")
	}
}

func TestAESGCMEncryptorInvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	encryptor, err := NewAESGCMEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 测试无效密文
	invalidCiphertexts := [][]byte{
		[]byte("tooshort"),
		[]byte("invalidciphertextthatistoolongbutinvalid"),
	}

	for _, ciphertext := range invalidCiphertexts {
		_, err := encryptor.Decrypt(ciphertext)
		if err == nil {
			t.Error("Expected error for invalid ciphertext, got nil")
		}
	}
}

// BenchmarkAESGCMEncrypt 测试加密性能
func BenchmarkAESGCMEncrypt(b *testing.B) {
	key := make([]byte, 32)
	encryptor, _ := NewAESGCMEncryptor(key)
	plaintext := []byte("This is a benchmark test message for AES-GCM encryption")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.Encrypt(plaintext)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

// BenchmarkAESGCMDecrypt 测试解密性能
func BenchmarkAESGCMDecrypt(b *testing.B) {
	key := make([]byte, 32)
	encryptor, _ := NewAESGCMEncryptor(key)
	plaintext := []byte("This is a benchmark test message for AES-GCM encryption")
	ciphertext, _ := encryptor.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.Decrypt(ciphertext)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}
