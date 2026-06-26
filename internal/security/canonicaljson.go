// Package security 提供接口加签 + 加解密能力（RSA-OAEP + AES-GCM + RSA-PSS）。
//
// 与 spring-slumber-web/src/lib/security/canonical-json.ts 保持 byte-equal（RFC 8785）。
package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

// CanonicalJSON 把任意可 JSON 序列化的值序列化为 RFC 8785 规范化字符串。
//
// 规则：
//   - 对象键按 UTF-16 codepoint 升序排序
//   - NaN / Inf 抛错
//   - 数字保持 json.Number 精度（避免 float64 截断）
//
// 实现策略：先 json.Marshal 拿原始字节（含 float64 字面量），若是对象则解码为
// map[string]json.RawMessage 按 key 排序后逐字段重新规范化；若是数组则递归。
// 数字类型在 json.Marshal 后是标准字面量，与前端 JSON.stringify 一致。
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonicalJson: marshal: %w", err)
	}

	var decoded any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("canonicalJson: decode: %w", err)
	}

	var buf bytes.Buffer
	if err := writeCanonical(&buf, decoded); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeCanonical 递归写规范化 JSON。
func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		// 直接写原始数字字面量（前端 JSON.stringify 行为一致）。
		buf.WriteString(x.String())
	case string:
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(b)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("canonicalJson: non-finite number")
		}
		buf.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case []any:
		buf.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("canonicalJson: unsupported type %T", v)
	}
	return nil
}
