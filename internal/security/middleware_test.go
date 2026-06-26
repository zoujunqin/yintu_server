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

// TestMiddleware_EncryptRoundTrip 模拟客户端 → 服务端的完整 envelope 流程。
func TestMiddleware_EncryptRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kp, err := LoadOrGenerateKeyPair("", "")
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(Middleware(Config{
		KeyPair: kp,
		Skipper: func(c *gin.Context) bool { return strings.HasPrefix(c.Request.URL.Path, "/public") },
	}))
	r.POST("/echo", func(c *gin.Context) {
		// 业务 handler：直接 echo 解密后的 request body。
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"echo": body})
	})

	// 构造客户端请求。
	aesKey, iv, _ := GenerateAESKey()
	plain := []byte(`{"hello":"世界","n":42}`)
	ct, err := AESEncrypt(aesKey, iv, plain)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := RSAEncryptOAEP(kp.Public, aesKey)
	if err != nil {
		t.Fatal(err)
	}
	reqEnv := Envelope{
		Enc:  true,
		Data: EncodeB64(ct),
		IV:   EncodeB64(iv),
		Key:  EncodeB64(wrapped),
		Meta: Meta{TS: time.Now().UnixMilli(), Nonce: "n1", KID: "client-key-1"},
	}
	reqBody, _ := json.Marshal(reqEnv)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	if strings.Contains(w.Body.String(), "MARKER") {
		t.Fatalf("response should be pure JSON envelope, got: %s", w.Body.String())
	}

	// 验证响应是 envelope 格式。
	var respEnv Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &respEnv); err != nil {
		t.Fatalf("response not envelope: %v, body = %s", err, w.Body.String())
	}
	if !respEnv.Enc {
		t.Fatalf("response enc should be true, got false")
	}
	if respEnv.Sig == "" {
		t.Fatalf("response should be signed")
	}

	// 客户端解密响应。
	respIv := mustDecode(respEnv.IV)
	respCt := mustDecode(respEnv.Data)
	plainResp, err := AESDecrypt(aesKey, respIv, respCt)
	if err != nil {
		t.Fatalf("AESDecrypt response: %v", err)
	}
	if !bytes.Contains(plainResp, []byte(`"echo":`)) {
		t.Errorf("decrypted body missing echo: %s", plainResp)
	}

	// 验证签名。
	sig, _ := DecodeB64(respEnv.Sig)
	signInput, _ := respEnv.SignBytes()
	if err := RSAVerifyPSS(kp.Public, signInput, sig); err != nil {
		t.Fatalf("RSAVerifyPSS: %v", err)
	}
}

// TestMiddleware_NonceReplayRejected 重复请求 envelope 会被拒绝，防止窗口内重放。
func TestMiddleware_NonceReplayRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kp, err := LoadOrGenerateKeyPair("", "")
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(Middleware(Config{KeyPair: kp}))
	r.POST("/echo", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	aesKey, iv, _ := GenerateAESKey()
	plain := []byte(`{"hello":"replay"}`)
	ct, err := AESEncrypt(aesKey, iv, plain)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := RSAEncryptOAEP(kp.Public, aesKey)
	if err != nil {
		t.Fatal(err)
	}
	replayNonce := "replay-nonce"
	reqEnv := Envelope{
		Enc:  true,
		Data: EncodeB64(ct),
		IV:   EncodeB64(iv),
		Key:  EncodeB64(wrapped),
		Meta: Meta{TS: time.Now().UnixMilli(), Nonce: replayNonce, KID: "client-key-1"},
	}
	reqBody, _ := json.Marshal(reqEnv)

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(reqBody))
	firstReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(first, firstReq)
	if first.Code != 200 {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(reqBody))
	secondReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(second, secondReq)
	if second.Code != 400 {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "nonce already used") {
		t.Errorf("expected nonce replay error, got: %s", second.Body.String())
	}
}

func TestMiddleware_PlaintextPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kp, _ := LoadOrGenerateKeyPair("", "")

	r := gin.New()
	r.Use(Middleware(Config{KeyPair: kp, Skipper: func(c *gin.Context) bool { return true }}))
	r.GET("/public", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("plain body missing: %s", w.Body.String())
	}
}

// TestMiddleware_TimestampRejected 过期 envelope 拒绝。
func TestMiddleware_TimestampRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kp, _ := LoadOrGenerateKeyPair("", "")

	r := gin.New()
	r.Use(Middleware(Config{KeyPair: kp}))
	r.POST("/echo", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// ts 设为 10 分钟前 → 超过 ±5min 窗口。
	env := Envelope{
		Meta: Meta{TS: time.Now().Add(-10 * time.Minute).UnixMilli(), Nonce: "old", KID: "k"},
	}
	body, _ := json.Marshal(env)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "timestamp out of window") {
		t.Errorf("expected timestamp error, got: %s", w.Body.String())
	}
}
