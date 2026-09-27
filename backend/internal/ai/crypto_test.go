package ai

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-secret-key"
	plain := "sk-abcdefghijklmnopqrstuvwxyz0123456789"

	cipher, err := EncryptSecret(secret, plain)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if cipher == "" || strings.Contains(cipher, plain) {
		t.Fatalf("密文异常: %q", cipher)
	}

	back, err := DecryptSecret(secret, cipher)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if back != plain {
		t.Fatalf("往返不一致: %q", back)
	}

	// 同一明文两次加密应产生不同密文（随机 nonce）
	cipher2, err := EncryptSecret(secret, plain)
	if err != nil {
		t.Fatalf("二次加密失败: %v", err)
	}
	if cipher == cipher2 {
		t.Error("两次加密结果相同，随机 nonce 可能失效")
	}
}

func TestDecryptWithWrongSecretFails(t *testing.T) {
	cipher, err := EncryptSecret("secret-a", "sk-test")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if _, err := DecryptSecret("secret-b", cipher); err == nil {
		t.Fatal("换了主密钥竟然解密成功，说明没有真正校验")
	}
}

func TestDecryptTamperedCipherFails(t *testing.T) {
	cipher, err := EncryptSecret("secret", "sk-test")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	// 篡改最后一个字符
	tampered := cipher[:len(cipher)-1] + "A"
	if tampered == cipher {
		tampered = cipher[:len(cipher)-1] + "B"
	}
	if _, err := DecryptSecret("secret", tampered); err == nil {
		t.Fatal("篡改后的密文竟然解密成功，GCM 校验可能失效")
	}
}

func TestEncryptRequiresSecret(t *testing.T) {
	if _, err := EncryptSecret("   ", "sk-test"); err == nil {
		t.Fatal("未配置主密钥时应报错")
	}
}

func TestEmptyPlaintextIsAllowed(t *testing.T) {
	cipher, err := EncryptSecret("secret", "")
	if err != nil {
		t.Fatalf("空明文应允许: %v", err)
	}
	if cipher != "" {
		t.Fatalf("空明文应得到空密文，实际 %q", cipher)
	}
	back, err := DecryptSecret("secret", "")
	if err != nil || back != "" {
		t.Fatalf("空密文应解出空串: %q %v", back, err)
	}
}

func TestMaskHintOnlyKeepsTail(t *testing.T) {
	if got := MaskHint("sk-1234567890abcd"); got != "****abcd" {
		t.Errorf("应只保留尾 4 位，实际 %q", got)
	}
	if got := MaskHint("abc"); got != "****" {
		t.Errorf("过短的密钥应整体打码，实际 %q", got)
	}
}
