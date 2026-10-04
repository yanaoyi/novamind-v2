package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

func TestEncryptDecryptV2RoundTrip(t *testing.T) {
	secret := "test-master-secret"
	cipher1, err := EncryptSecret(secret, "sk-abcdef123456")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if !strings.HasPrefix(cipher1, cipherV2Prefix) {
		t.Fatalf("新密文应带 %s 前缀，实际 %q", cipherV2Prefix, cipher1)
	}
	plain, err := DecryptSecret(secret, cipher1)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if plain != "sk-abcdef123456" {
		t.Fatalf("解密结果不对: %q", plain)
	}
	// 带盐：同一明文两次加密结果必须不同（否则等于没加盐）
	cipher2, _ := EncryptSecret(secret, "sk-abcdef123456")
	if cipher1 == cipher2 {
		t.Error("两次加密结果相同，说明盐没有生效")
	}
	if _, err := DecryptSecret("wrong-secret", cipher1); err == nil {
		t.Error("主密钥不对时必须解密失败")
	}
}

func TestDecryptLegacyV1Cipher(t *testing.T) {
	secret := "test-master-secret"
	legacy := legacyEncrypt(t, secret, "sk-legacy-key")
	if !IsLegacyCipher(legacy) {
		t.Fatal("没有前缀的密文应被识别为 v1")
	}
	plain, err := DecryptSecret(secret, legacy)
	if err != nil {
		t.Fatalf("v1 历史密文必须仍能解开（升级不能架空老数据）: %v", err)
	}
	if plain != "sk-legacy-key" {
		t.Fatalf("v1 解密结果不对: %q", plain)
	}
	if IsLegacyCipher("v2:whatever") {
		t.Error("v2 密文不应被识别为 v1")
	}
	if IsLegacyCipher("") {
		t.Error("空密文不算 v1")
	}
}

// legacyEncrypt 复刻升级前的实现（裸 SHA-256 当 AES 密钥 + base64(nonce||ct)），
// 保证"老数据仍可读"这件事有真实用例兜着。
func legacyEncrypt(t *testing.T, secret, plaintext string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("初始化 GCM 失败: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("随机数失败: %v", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed)
}
