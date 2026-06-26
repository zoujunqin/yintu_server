package security

import (
	"bytes"
	"testing"
)

// TestEnvelopeRoundTrip 验证 RSA 包 AES key + AES-GCM 加解密 + RSA-PSS 签验签的端到端流程。
func TestEnvelopeRoundTrip(t *testing.T) {
	kp, err := LoadOrGenerateKeyPair("", "")
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte(`{"hello":"世界","list":[1,2,3]}`)

	// 1. 客户端生成 AES key + IV，加密。
	aesKey, iv, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	ct, err := AESEncrypt(aesKey, iv, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 用服务端公钥包 AES key。
	wrapped, err := RSAEncryptOAEP(kp.Public, aesKey)
	if err != nil {
		t.Fatal(err)
	}

	// 3. 服务端解开 AES key，解密 payload。
	gotKey, err := RSADecryptOAEP(kp.Private, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotKey, aesKey) {
		t.Fatal("unwrapped key mismatch")
	}
	gotPlain, err := AESDecrypt(gotKey, iv, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPlain, plaintext) {
		t.Errorf("plaintext mismatch: got %s, want %s", gotPlain, plaintext)
	}

	// 4. 服务端签名响应 envelope。
	env := Envelope{
		Enc:  true,
		Data: EncodeB64(ct),
		IV:   EncodeB64(iv),
		Meta: Meta{TS: 1700000000000, Nonce: "abc", KID: "server-key-1"},
	}
	signInput, err := env.SignBytes()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := RSASignPSS(kp.Private, signInput)
	if err != nil {
		t.Fatal(err)
	}
	env.Sig = EncodeB64(sig)

	// 5. 客户端验签。
	sigBytes, _ := DecodeB64(env.Sig)
	verifyInput, _ := env.SignBytes()
	if err := RSAVerifyPSS(kp.Public, verifyInput, sigBytes); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

// TestCanonicalJSON_Deterministic 同一输入反复输出 byte-equal。
func TestCanonicalJSON_Deterministic(t *testing.T) {
	in := map[string]any{"z": 1, "a": "x", "nested": map[string]any{"y": 2, "b": true}}
	first, _ := CanonicalJSON(in)
	for i := 0; i < 10; i++ {
		got, _ := CanonicalJSON(in)
		if !bytes.Equal(first, got) {
			t.Fatalf("non-deterministic at iter %d", i)
		}
	}
}
