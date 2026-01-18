package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryptor 加密器接口
type Encryptor interface {
	// Encrypt 加密数据
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt 解密数据
	Decrypt(ciphertext []byte) ([]byte, error)

	// EncryptToString 加密并返回 Base64 编码的字符串
	EncryptToString(plaintext []byte) (string, error)

	// DecryptFromString 从 Base64 编码的字符串解密
	DecryptFromString(ciphertext string) ([]byte, error)
}

// AESGCMEncryptor AES-GCM 加密器
type AESGCMEncryptor struct {
	key []byte
}

// NewAESGCMEncryptor 创建 AES-GCM 加密器
// key 必须是 16, 24, 或 32 字节，分别对应 AES-128, AES-192, 或 AES-256
func NewAESGCMEncryptor(key []byte) (*AESGCMEncryptor, error) {
	keySize := len(key)
	if keySize != 16 && keySize != 24 && keySize != 32 {
		return nil, fmt.Errorf("invalid key size: %d (must be 16, 24, or 32 bytes)", keySize)
	}

	return &AESGCMEncryptor{
		key: key,
	}, nil
}

// Encrypt 加密数据
func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密数据（nonce 会前置到密文）
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 解密数据
func (e *AESGCMEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// 提取 nonce 和实际密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptToString 加密并返回 Base64 编码的字符串
func (e *AESGCMEncryptor) EncryptToString(plaintext []byte) (string, error) {
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptFromString 从 Base64 编码的字符串解密
func (e *AESGCMEncryptor) DecryptFromString(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return e.Decrypt(data)
}
