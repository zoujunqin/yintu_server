package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestKeygenRoundTrip 模拟 cmd/keygen 输出的 PEM 格式能被安全模块加载并参与 envelope 流程。
func TestKeygenRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 1. 生成一对 keypair，并按 keygen 格式导出 PEM + SPKI base64。
	kp, err := LoadOrGenerateKeyPair("", "")
	if err != nil {
		t.Fatal(err)
	}
	privPEM, err := MarshalPrivateKeyPEM(kp.Private)
	if err != nil {
		t.Fatal(err)
	}
	pubB64, err := MarshalPublicKeySPKI(kp.Public)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 模拟从 .env 读取：把 PEM 当 SIGN_PRIVATE_KEY、SPKI 当 SIGN_PUBLIC_KEY 加载。
	reloaded, err := LoadOrGenerateKeyPair(privPEM, pubB64)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeyPair: %v", err)
	}

	// 3. 端到端：envelope 请求 → 中间件解密 → handler 处理 → 中间件加密 + 签名 envelope 响应。
	r := gin.New()
	r.Use(Middleware(Config{KeyPair: reloaded}))
	r.POST("/echo", func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"echo": body})
	})

	aesKey, iv, _ := GenerateAESKey()
	plain := []byte(`{"hello":"keygen"}`)
	ct, _ := AESEncrypt(aesKey, iv, plain)
	wrapped, _ := RSAEncryptOAEP(reloaded.Public, aesKey)
	reqEnv := Envelope{
		Enc:  true,
		Data: EncodeB64(ct),
		IV:   EncodeB64(iv),
		Key:  EncodeB64(wrapped),
		Meta: Meta{TS: time.Now().UnixMilli(), Nonce: "e2e", KID: "client-key-1"},
	}
	body, _ := json.Marshal(reqEnv)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var respEnv Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &respEnv); err != nil {
		t.Fatalf("unmarshal response envelope: %v, body = %s", err, w.Body.String())
	}
	if !respEnv.Enc {
		t.Errorf("response enc should be true")
	}
	if respEnv.Sig == "" {
		t.Errorf("response should be signed")
	}
	if strings.Contains(string(w.Body.Bytes()), "\"error\":") {
		t.Errorf("response contains error: %s", w.Body.String())
	}
}
