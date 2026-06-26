// Package security RSA 密钥管理。
package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// KeyPair 同时持有 RSA 私钥 + 公钥。
type KeyPair struct {
	Private *rsa.PrivateKey
	Public  *rsa.PublicKey
}

// LoadOrGenerateKeyPair 优先从 SIGN_PRIVATE_KEY（PEM 编码）加载私钥；
// 未配置时生成一个全新的 RSA-2048 keypair（仅 dev / 启动场景使用）。
//
// SIGN_PRIVATE_KEY 格式：PEM block "RSA PRIVATE KEY"（PKCS#1）或 "PRIVATE KEY"（PKCS#8）。
// 公钥格式：可通过 env SIGN_PUBLIC_KEY 注入 SPKI base64，供 /security/public-key 暴露。
func LoadOrGenerateKeyPair(privPEM string, pubSPKIBase64 string) (*KeyPair, error) {
	if privPEM != "" {
		priv, err := parsePrivateKey(privPEM)
		if err != nil {
			return nil, fmt.Errorf("parse SIGN_PRIVATE_KEY: %w", err)
		}
		pub, err := resolvePublicKey(pubSPKIBase64, priv)
		if err != nil {
			return nil, err
		}
		return &KeyPair{Private: priv, Public: pub}, nil
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("rsa.GenerateKey: %w", err)
	}
	return &KeyPair{Private: priv, Public: &priv.PublicKey}, nil
}

// parsePrivateKey 同时支持 PKCS#1 与 PKCS#8 PEM。
func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM data")
	}
	if priv, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return priv, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8: %w", err)
	}
	priv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("PKCS#8 key is not RSA")
	}
	return priv, nil
}

// resolvePublicKey 优先用 env 注入的 SPKI base64；缺失时从私钥推导。
func resolvePublicKey(spkiBase64 string, priv *rsa.PrivateKey) (*rsa.PublicKey, error) {
	if spkiBase64 == "" {
		return &priv.PublicKey, nil
	}
	der, err := base64.StdEncoding.DecodeString(spkiBase64)
	if err != nil {
		return nil, fmt.Errorf("decode SIGN_PUBLIC_KEY: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse SPKI: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("SPKI key is not RSA")
	}
	return rsaPub, nil
}

// MarshalPublicKeySPKI 把 RSA 公钥序列化为 SPKI DER → base64。
func MarshalPublicKeySPKI(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// MarshalPrivateKeyPEM 把私钥导出为 PKCS#8 PEM（便于落盘 / 注入环境变量）。
func MarshalPrivateKeyPEM(priv *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

// ReadEnvFile 读取 env 文件并返回 key=value 字典（用于 keygen 工具）。
func ReadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range rangeLines(string(data)) {
		if line == "" || line[0] == '#' {
			continue
		}
		eq := indexByte(line, '=')
		if eq < 0 {
			continue
		}
		out[line[:eq]] = line[eq+1:]
	}
	return out, nil
}

// rangeLines / indexByte：避免引入 strings 包（小工具内联）。
func rangeLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
