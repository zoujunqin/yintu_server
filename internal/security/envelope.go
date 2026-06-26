// Package security 加签 + 加解密 envelope 数据结构。
//
// 与 spring-slumber-web/src/types/security.ts 中的 IEnvelope 字段保持一一对应。
package security

import "encoding/json"

// Envelope 接口加签 + 加解密信封。
//
// 字段语义：
//   - Enc:       后端写入；true 表示 data 已用 AES-GCM 加密
//   - Data:      base64(AES-GCM 密文 || 16B tag)，Enc=true 时存在
//   - IV:        base64(12B GCM nonce)
//   - Tag:       可选独立 tag 字段（默认已合并到 Data 尾部，保留以兼容自定义协议）
//   - Key:       base64(RSA-OAEP 包裹的 AES key)，仅请求方向存在
//   - Meta:      ts / nonce / kid 三元组，用于防重放
//   - Sig:       base64(RSA-PSS 签名)，仅响应方向存在
type Envelope struct {
	Enc  bool   `json:"enc"`
	Data string `json:"data,omitempty"`
	IV   string `json:"iv,omitempty"`
	Tag  string `json:"tag,omitempty"`
	Key  string `json:"key,omitempty"`
	Meta Meta   `json:"meta"`
	Sig  string `json:"sig,omitempty"`
}

// Meta 信封元数据。
type Meta struct {
	TS    int64  `json:"ts"`    // Unix 毫秒
	Nonce string `json:"nonce"` // 客户端随机串
	KID   string `json:"kid"`   // 当前签名 / 加密密钥标识
}

// SignedPayload 签名输入的规范化结构（不含 Sig 自身）。
//
// 客户端构造请求时：{Enc, Data, IV, Key, Meta}
// 服务端构造响应时：{Enc, Data, IV, Key, Meta}（Key 留空字符串，与前端 canonicalJson 保持 byte-equal）
//
// 注意：Data/IV/Key 一律不带 omitempty，必须始终序列化（即使是空串）。
// 这与前端 canonicalJson 的"undefined 省略 / 空串保留"规则一致，保证两侧签名输入 byte-equal。
type SignedPayload struct {
	Enc  bool   `json:"enc"`
	Data string `json:"data"`
	IV   string `json:"iv"`
	Key  string `json:"key"`
	Meta Meta   `json:"meta"`
}

// SignBytes 返回签名输入的字节流（用于 RSA-PSS）。
func (e *Envelope) SignBytes() ([]byte, error) {
	payload := SignedPayload{
		Enc:  e.Enc,
		Data: e.Data,
		IV:   e.IV,
		Key:  e.Key,
		Meta: e.Meta,
	}
	return CanonicalJSON(payload)
}

// DecodeEnvelope 把任意 JSON 解码为 Envelope（结构嗅探，失败返回 nil）。
func DecodeEnvelope(raw json.RawMessage) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// CanonicalFields 返回签名输入固定字段列表（供单元测试 / 跨语言对齐校验）。
func CanonicalFields() []string {
	return []string{"data", "enc", "iv", "key", "meta"}
}
