package account

import (
	"os"
	"path/filepath"
	"sync"
)

// 数据根目录：默认 ~/.qoder2api
// 可通过 QODER2API_HOME 或 SetDataRoot 覆盖，便于 CN / Global 双实例隔离。
var (
	dataRootMu sync.RWMutex
	dataRoot   string
)

func SetDataRoot(dir string) {
	dataRootMu.Lock()
	defer dataRootMu.Unlock()
	dataRoot = dir
}

func DataRoot() string {
	dataRootMu.RLock()
	if dataRoot != "" {
		r := dataRoot
		dataRootMu.RUnlock()
		return r
	}
	dataRootMu.RUnlock()

	if v := os.Getenv("QODER2API_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".qoder2api"
	}
	return filepath.Join(home, ".qoder2api")
}

func accountsDir() string {
	return filepath.Join(DataRoot(), "accounts")
}

func settingsPath() string {
	return filepath.Join(DataRoot(), "settings.json")
}

func SecretsDir() string {
	return filepath.Join(DataRoot(), "secrets")
}

func LogsDir() string {
	return filepath.Join(DataRoot(), "logs")
}
