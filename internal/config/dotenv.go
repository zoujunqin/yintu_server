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

func trimQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}

	return value
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
