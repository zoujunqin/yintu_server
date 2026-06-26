// Package security Gin 中间件：双向加签 + 加解密（RSA-OAEP + AES-GCM + RSA-PSS）。
//
// 入站（客户端 → 服务端）：
//   - 嗅 envelope 格式；命中后 → 验时间戳 + RSA-OAEP 解包 AES key + AES-GCM 解密 body
//   - 还原 c.Request.Body 为解密后的明文 JSON，业务 handler 透明处理
//
// 出站（服务端 → 客户端）：
//   - 通过 buffering ResponseWriter 捕获 handler 写入的原始 body
//   - 用请求 envelope 携带的 AES key 加密响应（同一会话复用）
//   - 用服务端 RSA 私钥对 envelope 做 RSA-PSS 签名
//   - 若 EncryptDisabled=true（dev），跳过加密但保留 envelope 壳 + 签名
//
// 公开端点：/healthz / /security/public-key / Swagger 通过 Skipper 跳过中间件。
package security

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKey gin.Context 上 Session 的键名。
	ContextKey = "security.session"
	// HeaderNonce 客户端 nonce header。
	HeaderNonce = "X-Request-Nonce"
	// TimestampWindow 时间戳容许漂移（与前端 timestamp-window.ts 保持一致：±5 分钟）。
	TimestampWindow = 5 * time.Minute
)

var inboundNonceStore = newNonceStore(TimestampWindow * 2)

// Session 单次请求的安全会话状态。
type Session struct {
	Envelope *Envelope // 解密后的入站 envelope
	AESKey   []byte    // 当前请求复用的 AES key；nil 表示未加密
	Plain    bool      // 入站为明文（dev / 公开端点）
}

// Config 中间件配置。
type Config struct {
	KeyPair         *KeyPair
	EncryptDisabled bool                    // dev 跳过加密
	SignDisabled    bool                    // dev 跳过签名
	Skipper         func(*gin.Context) bool // 公开端点跳过
}

// Middleware 返回双向加签 + 加解密中间件。
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.Skipper == nil {
		cfg.Skipper = func(*gin.Context) bool { return false }
	}
	return func(c *gin.Context) {
		if cfg.Skipper(c) {
			c.Next()
			return
		}

		bw := &bufferWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = bw

		sess := &Session{}
		if err := decryptInbound(c, cfg, sess); err != nil {
			abortError(c, http.StatusBadRequest, "decrypt_failed", err.Error())
			return
		}
		c.Set(ContextKey, sess)

		c.Next()

		if !bw.wroteHeader() {
			return
		}
		if err := encryptOutbound(c, cfg, sess, bw.body.Bytes()); err != nil {
			abortError(c, http.StatusInternalServerError, "encrypt_failed", err.Error())
			return
		}
	}
}

// decryptInbound 解密入站请求；填充 sess.Envelope / sess.AESKey。
func decryptInbound(c *gin.Context, cfg Config, sess *Session) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil || !looksLikeEnvelope(&env) {
		sess.Plain = true
		return nil
	}
	if abs(time.Since(time.UnixMilli(env.Meta.TS))) > TimestampWindow {
		return errors.New("timestamp out of window")
	}
	if !inboundNonceStore.Remember(env.Meta.Nonce, env.Meta.TS) {
		return errors.New("nonce already used")
	}
	sess.Envelope = &env

	if env.Enc && !cfg.EncryptDisabled && env.Key != "" {
		wrapped, err := DecodeB64(env.Key)
		if err != nil {
			return err
		}
		key, err := RSADecryptOAEP(cfg.KeyPair.Private, wrapped)
		if err != nil {
			return err
		}
		if len(key) != AESKeyLen {
			return errors.New("unwrapped AES key length invalid")
		}
		sess.AESKey = key

		iv := mustDecode(env.IV)
		ct := mustDecode(env.Data)
		plain, err := AESDecrypt(key, iv, ct)
		if err != nil {
			return err
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(plain))
		c.Request.ContentLength = int64(len(plain))
	}
	return nil
}

// encryptOutbound 把 handler 写入的 body 包装为 envelope：加密 + 签名。
func encryptOutbound(c *gin.Context, cfg Config, sess *Session, body []byte) error {
	out := Envelope{
		Meta: Meta{
			TS:    time.Now().UnixMilli(),
			Nonce: nonceFrom(sess),
			KID:   "server-key-1",
		},
	}
	if !cfg.EncryptDisabled && sess.AESKey != nil {
		iv, err := randomBytes(GCMNonceLen)
		if err != nil {
			return err
		}
		ct, err := AESEncrypt(sess.AESKey, iv, body)
		if err != nil {
			return err
		}
		out.Enc = true
		out.IV = EncodeB64(iv)
		out.Data = EncodeB64(ct)
	} else {
		out.Enc = false
		out.Data = string(body)
	}

	if !cfg.SignDisabled && cfg.KeyPair != nil {
		signInput, err := out.SignBytes()
		if err != nil {
			return err
		}
		sig, err := RSASignPSS(cfg.KeyPair.Private, signInput)
		if err != nil {
			return err
		}
		out.Sig = EncodeB64(sig)
	}

	// 写入原始 ResponseWriter（不是 bufferWriter，否则 envelope 会回灌 buffer）。
	rw := underlyingWriter(c.Writer)
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	envBytes, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = rw.Write(envBytes)
	return err
}

// underlyingWriter 取出 bufferWriter 包裹的原始 gin.ResponseWriter。
func underlyingWriter(w gin.ResponseWriter) gin.ResponseWriter {
	if bw, ok := w.(*bufferWriter); ok {
		return bw.ResponseWriter
	}
	return w
}

// ── buffering ResponseWriter ──

// bufferWriter 捕获 handler 写入的 body；headers 透传，body 缓存到内存。
// encryptOutbound 在 c.Next() 之后读 body 重新包装 envelope。
type bufferWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	wroteFlag bool
}

func (w *bufferWriter) Write(b []byte) (int, error) {
	w.wroteFlag = true
	return w.body.Write(b)
}

func (w *bufferWriter) WriteString(s string) (int, error) {
	w.wroteFlag = true
	return w.body.WriteString(s)
}

func (w *bufferWriter) wroteHeader() bool { return w.wroteFlag }

// ── helpers ──

func looksLikeEnvelope(e *Envelope) bool {
	if e == nil {
		return false
	}
	if e.Meta.TS == 0 || e.Meta.Nonce == "" || e.Meta.KID == "" {
		return false
	}
	return true
}

func mustDecode(s string) []byte {
	b, _ := DecodeB64(s)
	return b
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// nonceStore 记录窗口内已消费的入站 nonce，防止请求 envelope 被重放。
type nonceStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]int64
}

// newNonceStore 创建入站 nonce 缓存。
func newNonceStore(ttl time.Duration) *nonceStore {
	return &nonceStore{ttl: ttl, entries: make(map[string]int64)}
}

// Remember 记录 nonce；已存在且未过期时返回 false。
func (s *nonceStore) Remember(nonce string, ts int64) bool {
	if nonce == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	s.gc(now)
	if _, exists := s.entries[nonce]; exists {
		return false
	}
	s.entries[nonce] = ts
	return true
}

func (s *nonceStore) gc(now int64) {
	cutoff := now - s.ttl.Milliseconds()
	for nonce, ts := range s.entries {
		if ts < cutoff {
			delete(s.entries, nonce)
		}
	}
}

func nonceFrom(sess *Session) string {
	if sess.Envelope != nil {
		return sess.Envelope.Meta.Nonce
	}
	return ""
}

func abortError(c *gin.Context, status int, code, msg string) {
	c.Abort()
	rw := underlyingWriter(c.Writer)
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(gin.H{
		"error": gin.H{"code": code, "message": msg},
	})
}

// SessionOf 业务 handler 取 session。
func SessionOf(c *gin.Context) *Session {
	v, _ := c.Get(ContextKey)
	s, _ := v.(*Session)
	return s
}

// MustPublicKey 暴露给 /security/public-key handler 返回公钥。
func MustPublicKey(kp *KeyPair) *rsa.PublicKey { return kp.Public }
