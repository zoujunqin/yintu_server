package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"spring-slumber-server/internal/config"
)

// TestKeygenDotEnvRoundTrip 覆盖 cmd/keygen → .env → config.Load() → security 整条链路。
//
// 这是对 keygen_e2e_test.go 的补充：原 e2e 直接 MarshalPrivateKeyPEM 注入，
// 跳过了 dotenv 的转义解码路径，因此一直没发现 SIGN_PRIVATE_KEY 在 .env 中字面 \n 的问题。
//
// 测试流程：
//  1. 生成 keypair，导出真实多行 PEM；
//  2. 模拟 cmd/keygen renderEnvBlock 的格式（PEM 中的真实换行 → 字面 \n，双引号包裹）；
//  3. 写到临时目录的 .env.test，chdir 后调 config.Load() 走与生产同一条加载链；
//  4. 断言 config 解析后的 SIGN_PRIVATE_KEY 已含真实换行；
//  5. 喂给 LoadOrGenerateKeyPair，验证可加载且与原私钥一致。
func TestKeygenDotEnvRoundTrip(t *testing.T) {
	// 1. 生成 keypair。
	kp, err := LoadOrGenerateKeyPair("", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	privPEM, err := MarshalPrivateKeyPEM(kp.Private)
	if err != nil {
		t.Fatalf("marshal priv: %v", err)
	}
	pubB64, err := MarshalPublicKeySPKI(kp.Public)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}

	// 2. 模拟 cmd/keygen renderEnvBlock：真实换行 → 字面 \n，双引号包裹。
	privEscaped := strings.ReplaceAll(strings.TrimSpace(privPEM), "\n", `\n`)
	dotenvContent := fmt.Sprintf(
		"SIGN_PRIVATE_KEY=\"%s\"\nSIGN_PUBLIC_KEY=\"%s\"\n",
		privEscaped, pubB64,
	)

	// 3. 写到临时目录并 chdir，让 config.Load() 走 .env.test。
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	envPath := filepath.Join(dir, ".env.test")
	if err := os.WriteFile(envPath, []byte(dotenvContent), 0o600); err != nil {
		t.Fatalf("write .env.test: %v", err)
	}

	// APP_ENV=test 让 Load() 加载 .env.test。
	// t.Setenv 自动还原:避免影响同包后续测试。
	t.Setenv("APP_ENV", "test")

	// 4. 真实加载链：config.Load() → cfg.Security.SignPrivateKey 应为合法 PEM。
	cfg := config.Load()
	loadedPEM := cfg.Security.SignPrivateKey
	if !strings.Contains(loadedPEM, "\n-----END PRIVATE KEY-----") {
		t.Fatalf("dotenv loader did not decode literal \\n in SIGN_PRIVATE_KEY:\n%s", loadedPEM)
	}

	// 5. security 真实加载 + 私钥比对。
	reloaded, err := LoadOrGenerateKeyPair(loadedPEM, cfg.Security.SignPublicKey)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeyPair after dotenv: %v", err)
	}
	if reloaded.Private.D.Cmp(kp.Private.D) != 0 {
		t.Fatalf("private key mismatch after dotenv round-trip")
	}
}
