package security

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestCrossLang_CanonicalResponseByteEqual 锁定后端 canonical JSON 输出。
// 前端 src/lib/security/signature.ts::rsaVerifyEnvelope 必须产生 byte-equal 输入。
//
// 响应信封示例：
//
//	{ enc: true, data: "AAA", iv: "BBB", key: "", meta: { ts: 1700000000000, nonce: "n", kid: "k" } }
//
// 期望 canonical JSON（key 按字典序）：
//
//	{"data":"AAA","enc":true,"iv":"BBB","key":"","meta":{"kid":"k","nonce":"n","ts":1700000000000}}
func TestCrossLang_CanonicalResponseByteEqual(t *testing.T) {
	env := Envelope{
		Enc:  true,
		Data: "AAA",
		IV:   "BBB",
		Key:  "",
		Meta: Meta{TS: 1700000000000, Nonce: "n", KID: "k"},
	}
	got, err := env.SignBytes()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":"AAA","enc":true,"iv":"BBB","key":"","meta":{"kid":"k","nonce":"n","ts":1700000000000}}`
	if string(got) != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
}

// TestCrossLang_CanonicalRequestByteEqual 锁定请求侧 canonical JSON 输出。
// 前端 src/lib/security/signature.ts::encryptRequestEnvelope 必须产生 byte-equal 输入。
//
// 请求信封示例：
//
//	{ enc: true, data: "CIPHER", iv: "IVV", key: "WRAPPED", meta: {...} }
func TestCrossLang_CanonicalRequestByteEqual(t *testing.T) {
	env := Envelope{
		Enc:  true,
		Data: "CIPHER",
		IV:   "IVV",
		Key:  "WRAPPED",
		Meta: Meta{TS: 1700000000000, Nonce: "n", KID: "client-key-1"},
	}
	got, err := env.SignBytes()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"data":"CIPHER","enc":true,"iv":"IVV","key":"WRAPPED","meta":{"kid":"client-key-1","nonce":"n","ts":1700000000000}}`
	if string(got) != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
}

// TestCrossLang_FieldOrderAlphabetical 验证 canonical 字段顺序按字典序。
// 防止有人重新引入 struct 字段顺序（非字典序）导致签名不一致。
func TestCrossLang_FieldOrderAlphabetical(t *testing.T) {
	env := Envelope{
		Enc:  true,
		Data: "x",
		IV:   "y",
		Key:  "z",
		Meta: Meta{TS: 1, Nonce: "n", KID: "k"},
	}
	got, _ := env.SignBytes()
	// meta 内部也按字典序：kid < nonce < ts
	if !strings.HasPrefix(string(got), `{"data":`) {
		t.Errorf("first field must be 'data', got: %s", got)
	}
	if !strings.Contains(string(got), `"meta":{"kid":`) {
		t.Errorf("meta first field must be 'kid', got: %s", got)
	}
}

// TestCrossLang_NoTrailingNewlineOrSpaces 防止有人加格式化字符破坏签名。
func TestCrossLang_NoTrailingNewlineOrSpaces(t *testing.T) {
	env := Envelope{
		Enc:  true,
		Data: "x",
		IV:   "y",
		Key:  "z",
		Meta: Meta{TS: 1, Nonce: "n", KID: "k"},
	}
	got, _ := env.SignBytes()
	if bytes.Contains(got, []byte{' '}) || bytes.Contains(got, []byte{'\n'}) {
		t.Errorf("canonical JSON must have no whitespace: %s", got)
	}
}

// TestCrossLang_RawMarshalRoundTrip 验证 json.Marshal 后重新规范化仍 byte-equal。
func TestCrossLang_RawMarshalRoundTrip(t *testing.T) {
	env := Envelope{
		Enc:  true,
		Data: "x",
		IV:   "y",
		Key:  "z",
		Meta: Meta{TS: 1, Nonce: "n", KID: "k"},
	}
	first, _ := env.SignBytes()

	// 模拟"前端把 envelope JSON 字符串 → 后端重新解析 → 再规范化"路径。
	raw, _ := json.Marshal(env)
	var reloaded Envelope
	_ = json.Unmarshal(raw, &reloaded)
	second, _ := reloaded.SignBytes()

	if !bytes.Equal(first, second) {
		t.Errorf("canonical not stable across marshal/unmarshal:\nfirst:  %s\nsecond: %s", first, second)
	}
}
