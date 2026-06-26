// Package security 加解密原语：AES-256-GCM + RSA-OAEP-SHA256 + RSA-PSS-SHA256。
package security

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	// AESKeyLen AES-256 密钥长度（字节）。
	AESKeyLen = 32
	// GCMNonceLen GCM 推荐 nonce 长度（字节）。
	GCMNonceLen = 12
	// GCMTagLen GCM tag 长度（字节，附在密文末尾）。
	GCMTagLen = 16
)

// Ciphertext AES-GCM 加密产物。
type Ciphertext struct {
	Key  []byte // 32B AES key
	IV   []byte // 12B nonce
	Data []byte // ciphertext + 16B GCM tag（密文尾部追加）
}

// GenerateAESKey 生成 32B 随机 AES key + 12B nonce。
func GenerateAESKey() ([]byte, []byte, error) {
	key := make([]byte, AESKeyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, err
	}
	iv := make([]byte, GCMNonceLen)
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, err
	}
	return key, iv, nil
}

// AESEncrypt 用 AES-256-GCM 加密；返回 ciphertext 与 16B tag 合并的字节切片。
func AESEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	if len(key) != AESKeyLen {
		return nil, fmt.Errorf("AESEncrypt: key must be %d bytes", AESKeyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if gcm.NonceSize() != GCMNonceLen {
		return nil, fmt.Errorf("AESEncrypt: gcm nonce size %d, want %d", gcm.NonceSize(), GCMNonceLen)
	}
	if len(iv) != GCMNonceLen {
		return nil, fmt.Errorf("AESEncrypt: iv must be %d bytes", GCMNonceLen)
	}
	sealed := gcm.Seal(nil, iv, plaintext, nil)
	return sealed, nil // sealed 末尾 16B 即 GCM tag
}

// AESDecrypt 解密；自动剥离尾部 16B tag 后再 Open。
func AESDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	if len(key) != AESKeyLen {
		return nil, fmt.Errorf("AESDecrypt: key must be %d bytes", AESKeyLen)
	}
	if len(ciphertext) < GCMTagLen {
		return nil, errors.New("AESDecrypt: ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ciphertext, nil)
}

// RSAEncryptOAEP 用 RSA-OAEP-SHA256 加密（小数据，如 AES key）。
func RSAEncryptOAEP(pub *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plaintext, nil)
}

// RSADecryptOAEP 用 RSA-OAEP-SHA256 解密。
func RSADecryptOAEP(priv *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, ciphertext, nil)
}

// RSASignPSS 用 RSA-PSS-SHA256 对 message 签名。
//
// 注意：传入的 message 是原文，函数内部用 SHA-256 哈希（与前端 Web Crypto PSS 一致）。
func RSASignPSS(priv *rsa.PrivateKey, message []byte) ([]byte, error) {
	hashed := sha256.Sum256(message)
	return rsa.SignPSS(rand.Reader, priv, crypto.SHA256, hashed[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
	})
}

// RSAVerifyPSS 用 RSA-PSS-SHA256 验签（message 是原文，函数内部哈希）。
func RSAVerifyPSS(pub *rsa.PublicKey, message, signature []byte) error {
	hashed := sha256.Sum256(message)
	return rsa.VerifyPSS(pub, crypto.SHA256, hashed[:], signature, &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
	})
}

// EncodeB64 base64 标准编码（用于 envelope 字段值）。
func EncodeB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// DecodeB64 base64 标准解码；空串返回 nil（不报错，便于可选字段）。
func DecodeB64(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// PublicKeyPEM 公钥转 PEM（便于人类阅读 / curl 测试）。
func PublicKeyPEM(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
