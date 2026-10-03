// Package ai 是 Model Gateway 与 Prompt Engine 的实现。
//
// 铁律（规格书 §36 / §64.13）：
//   - 业务层只能通过本包的 Gateway 调模型，不得直接依赖任何厂商 SDK；
//   - API Key 只以密文入库、只在内存中解密使用，绝不进日志或响应。
package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrMissingSecret 表示未配置用于加密密钥的主密钥。
var ErrMissingSecret = errors.New("未配置 NOVAMIND_SECRET，无法加密/解密模型密钥")

// deriveKey 用 SHA-256 从主密钥派生 32 字节 AES 密钥。
// 说明：主密钥是人手配置的口令，长度不定，用 KDF 归一比直接截断更稳妥。
func deriveKey(secret string) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrMissingSecret
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

// EncryptSecret 加密明文，返回 base64(nonce || ciphertext)。
func EncryptSecret(secret, plaintext string) (string, error) {
	key, err := deriveKey(secret)
	if err != nil {
		return "", err
	}
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret 解密 EncryptSecret 的结果。
func DecryptSecret(secret, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	key, err := deriveKey(secret)
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("密钥密文格式错误: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("密钥密文长度不足")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		// 解密失败通常意味着 NOVAMIND_SECRET 变了
		return "", errors.New("密钥解密失败：NOVAMIND_SECRET 可能已变更")
	}
	return string(plain), nil
}

// MaskHint 生成密钥提示（只保留尾 4 位，用于界面确认"填的是哪把"）。
// 注意：接口默认不返回它，只在明确需要时使用。
func MaskHint(plaintext string) string {
	runes := []rune(plaintext)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
