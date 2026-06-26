package config

import (
	"bufio"
	"os"
	"strings"
)

// loadDotEnv 从 path 读取 KEY=VALUE 行写入进程环境变量。
//
// protected 是进程环境变量在 Load() 启动时的快照键集合，这些键永远不被覆盖；
// 其它键会被直接覆盖，从而允许后续加载的 .env 文件覆盖先前加载的 .env 文件，
// 实现 .env.local > .env.{APP_ENV} > .env 的优先级。
//
// 空值会被跳过；如果用户想"清空"某个键，应当从文件中移除该行（dotenv 风格）。
func loadDotEnv(path string, protected map[string]bool) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}

		if protected[key] {
			continue
		}

		_ = os.Setenv(key, trimQuotes(value))
	}
}

// trimQuotes 去掉首尾匹配的引号（单/双），并按 dotenv 约定解码转义：
//
//   - 双引号包裹：支持 \n \r \t \\ \" 的转义解码（godotenv 行为）。
//   - 单引号包裹：字面量，不做任何解码。
//   - 无引号    ：原样返回。
//
// 解码能力是 cmd/keygen 把多行 PEM 折成单行 .env value 的前置条件：
// 不解码 \n 时，PEM 中的 \n 字面字符会让 pem.Decode 解析失败。
func trimQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	first, last := value[0], value[len(value)-1]
	switch {
	case first == '"' && last == '"':
		return unescapeDoubleQuoted(value[1 : len(value)-1])
	case first == '\'' && last == '\'':
		return value[1 : len(value)-1]
	default:
		return value
	}
}

// unescapeDoubleQuoted 解码双引号字符串中的转义序列（dotenv 标准子集）。
// 未知序列原样保留（如 \x），不做报错以兼容未来扩展。
func unescapeDoubleQuoted(value string) string {
	if !containsBackslash(value) {
		return value
	}

	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			switch value[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				b.WriteByte(value[i])
				b.WriteByte(value[i+1])
			}
			i++
			continue
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

func containsBackslash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			return true
		}
	}
	return false
}

// snapshotProcessEnv 捕获 Load() 调用前的进程环境变量键集合。
// 返回的 map 只关心键是否存在（用于保护），值留作后续扩展。
func snapshotProcessEnv() map[string]bool {
	protected := make(map[string]bool)
	for _, kv := range os.Environ() {
		if i := strings.Index(kv, "="); i >= 0 {
			protected[kv[:i]] = true
		}
	}
	return protected
}
