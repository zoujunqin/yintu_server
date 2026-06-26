package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDotenv 把 lines 写入临时 .env 文件并返回绝对路径。
// 测试结束由 t.Cleanup 负责删除。
func writeDotenv(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(joinLines(lines)), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	return path
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

// TestLoadDotEnv_EscapeDecoding 验证双引号包裹字符串中的转义序列被解码。
//
// 这是 dotenv 标准约定（参考 godotenv）：双引号支持 \n \r \t \\ \"；
// 单引号是字面量；无引号按原样。
func TestLoadDotEnv_EscapeDecoding(t *testing.T) {
	cases := []struct {
		name string
		line string // .env 中的一行
		key  string
		want string
	}{
		{
			name: "double-quoted newline",
			line: `KEY="line1\nline2"`,
			key:  "KEY",
			want: "line1\nline2",
		},
		{
			name: "double-quoted backslash",
			line: `KEY="a\\b"`,
			key:  "KEY",
			want: `a\b`,
		},
		{
			name: "double-quoted dquote",
			line: `KEY="say \"hi\""`,
			key:  "KEY",
			want: `say "hi"`,
		},
		{
			name: "double-quoted tab and cr literal escape",
			line: `KEY="col1\tcol2\rcol3"`,
			key:  "KEY",
			want: "col1\tcol2\rcol3",
		},
		{
			name: "single-quoted literal keeps backslash-n",
			line: `KEY="should not match"`,
			key:  "OTHER_KEY",
			want: "", // OTHER_KEY 不存在,Getenv 返回 ""
		},
		{
			name: "single-quoted literal newline",
			line: `KEY='literal\n'`,
			key:  "KEY",
			want: `literal\n`, // 单引号字面量,不解码
		},
		{
			name: "unquoted value",
			line: `KEY=plain`,
			key:  "KEY",
			want: "plain",
		},
		{
			name: "empty value skipped",
			line: `KEY=`,
			key:  "KEY",
			want: "", // 空值被跳过,Getenv 返回 ""
		},
		{
			name: "no quote no decode",
			line: `KEY=back\nslash`,
			key:  "KEY",
			want: `back\nslash`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 隔离：清掉可能受保护的进程 env，然后用 protected={} 强制覆盖
			t.Setenv(tc.key, "")
			protected := map[string]bool{} // 关闭保护,确保 .env 内容生效
			path := writeDotenv(t, tc.line)
			loadDotEnv(path, protected)

			got := os.Getenv(tc.key)
			if got != tc.want {
				t.Fatalf("env[%s] mismatch:\n want=%q\n got =%q", tc.key, tc.want, got)
			}
		})
	}
}

// TestLoadDotEnv_ProcessEnvProtected 验证 Load() 启动时已有的进程 env 不会被 .env 覆盖。
func TestLoadDotEnv_ProcessEnvProtected(t *testing.T) {
	t.Setenv("PROTECTED_KEY", "from-process")
	protected := map[string]bool{"PROTECTED_KEY": true}
	path := writeDotenv(t, "PROTECTED_KEY=from-dotenv")

	loadDotEnv(path, protected)

	if got := os.Getenv("PROTECTED_KEY"); got != "from-process" {
		t.Fatalf("protected env was overridden: got %q", got)
	}
}
