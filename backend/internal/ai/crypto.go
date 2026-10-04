// Package ai 是 Model Gateway 与 Prompt Engine 的实现。
//
// 铁律（规格书 §36 / §64.13）：
//   - 业务层只能通过本包的 Gateway 调模型，不得直接依赖任何厂商 SDK；
//   - API Key 只以密文入库、只在内存中解密使用，绝不进日志或响应。
package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
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
//
// 保留这个函数只为**解密历史密文**（v1 格式），新加密一律走 deriveKeyV2（审查 P2 的 KDF 升级）。
func deriveKey(secret string) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrMissingSecret
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

// 密文版本标记与 KDF 参数（v2）。
//
//   - v1（历史）：裸 SHA-256(secret) 当 AES-256 密钥，密文是 base64(nonce||ct)，无版本前缀；
//   - v2（现在）：HKDF-SHA256 派生（带随机盐 + info 上下文），密文前缀 "v2:"，
//     内容为 base64(salt || nonce || ciphertext)。
//
// 为什么要升级：裸 SHA-256 没有盐、没有迭代，弱主密钥可被暴力破解；
// HKDF 有盐且把用途绑定进 info，符合「不要把哈希当 KDF 用」的常识。
const (
	cipherV2Prefix = "v2:"
	hkdfSaltLen    = 16
	hkdfInfo       = "novamind/api-key/v2"
)

// deriveKeyV2 用 HKDF-SHA256 从主密钥 + 随机盐派生 32 字节 AES 密钥。
func deriveKeyV2(secret string, salt []byte) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrMissingSecret
	}
	// crypto/hkdf 是 Go 1.24+ 的标准库实现（不是 x/crypto 那个 Reader 版本）
	key, err := hkdf.Key(sha256.New, []byte(secret), salt, hkdfInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("派生密钥失败: %w", err)
	}
	return key, nil
}

// EncryptSecret 加密明文，返回 v2 格式密文（"v2:" + base64(salt||nonce||ciphertext)）。
func EncryptSecret(secret, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	salt := make([]byte, hkdfSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("生成盐失败: %w", err)
	}
	key, err := deriveKeyV2(secret, salt)
	if err != nil {
		return "", err
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
	return cipherV2Prefix + base64.StdEncoding.EncodeToString(append(salt, sealed...)), nil
}

// DecryptSecret 解密 EncryptSecret 的结果，同时兼容 v1 历史密文。
func DecryptSecret(secret, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if strings.HasPrefix(ciphertext, cipherV2Prefix) {
		return decryptV2(secret, strings.TrimPrefix(ciphertext, cipherV2Prefix))
	}
	return decryptV1(secret, ciphertext)
}

// IsLegacyCipher 判断密文是否为 v1 格式（用于"读到时顺手升级"）。
func IsLegacyCipher(ciphertext string) bool {
	return ciphertext != "" && !strings.HasPrefix(ciphertext, cipherV2Prefix)
}

func decryptV2(secret, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("密钥密文格式错误: %w", err)
	}
	if len(raw) < hkdfSaltLen {
		return "", errors.New("密钥密文长度不足")
	}
	salt, body := raw[:hkdfSaltLen], raw[hkdfSaltLen:]
	key, err := deriveKeyV2(secret, salt)
	if err != nil {
		return "", err
	}
	return openGCM(key, body)
}

func decryptV1(secret, encoded string) (string, error) {
	key, err := deriveKey(secret)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("密钥密文格式错误: %w", err)
	}
	return openGCM(key, raw)
}

// openGCM 用给定密钥解开 nonce||ciphertext。
func openGCM(key, raw []byte) (string, error) {
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
