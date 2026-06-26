package security

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCanonicalJSON_ObjectKeySort 验证对象键按字典序排序。
func TestCanonicalJSON_ObjectKeySort(t *testing.T) {
	got, err := CanonicalJSON(map[string]any{"b": 2, "a": 1, "c": 3})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":1,"b":2,"c":3}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestCanonicalJSON_Nested 验证嵌套对象 / 数组的规范化。
func TestCanonicalJSON_Nested(t *testing.T) {
	in := map[string]any{
		"z": []any{
			map[string]any{"b": 2, "a": 1},
			nil,
			true,
			"str",
			float64(3.14),
		},
		"meta": map[string]any{"ts": int64(1700000000000), "kid": "k1"},
	}
	got, err := CanonicalJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"meta":{"kid":"k1","ts":1700000000000},"z":[{"a":1,"b":2},null,true,"str",3.14]}`
	if string(got) != want {
		t.Errorf("\ngot:  %s\nwant: %s", got, want)
	}
}

// TestCanonicalJSON_NumberPreservation 验证大整数不被 float 截断。
func TestCanonicalJSON_NumberPreservation(t *testing.T) {
	// 通过 json.Number 输入规避 float64 精度损失。
	got, err := CanonicalJSON(map[string]any{
		"big": json.Number("9007199254740993"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"big":9007199254740993}` {
		t.Errorf("got %s", got)
	}
}

// TestCanonicalJSON_Escapes 验证字符串转义。
func TestCanonicalJSON_Escapes(t *testing.T) {
	got, err := CanonicalJSON(map[string]any{"k": "line1\nline2\t\"q\""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"line1\nline2\t\"q\""`) {
		t.Errorf("escape mismatch: %s", got)
	}
}

// TestCanonicalJSON_StructFieldOrder 验证 struct 字段按字典序输出。
func TestCanonicalJSON_StructFieldOrder(t *testing.T) {
	type S struct {
		Z int    `json:"z"`
		A string `json:"a"`
		M int    `json:"m"`
	}
	got, err := CanonicalJSON(S{Z: 1, A: "x", M: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"x","m":2,"z":1}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
