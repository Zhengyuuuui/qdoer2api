package account

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 文件存储密钥（服务器友好，不依赖 macOS Keychain）
// 路径: ~/.qoder2api/secrets/<id>.token
//
// 兼容从 QCCG Keychain 导出的值：
// - 明文 JSON: {"device_token":"dt-...","refresh_token":"drt-..."}
// - PAT 明文
// - go-keyring 前缀: go-keyring-base64:<base64>

const keyringPrefix = "go-keyring-base64:"

func secretsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".qoder2api", "secrets")
	return d, os.MkdirAll(d, 0700)
}

func secretPath(accountID string) (string, error) {
	d, err := secretsDir()
	if err != nil {
		return "", err
	}
	id := SanitizeID(accountID)
	if id == "" {
		return "", fmt.Errorf("empty account id")
	}
	return filepath.Join(d, id+".token"), nil
}

func normalizeSecret(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, keyringPrefix) {
		enc := strings.TrimPrefix(s, keyringPrefix)
		if b, err := base64.StdEncoding.DecodeString(enc); err == nil {
			s = strings.TrimSpace(string(b))
		} else if b, err := base64.RawStdEncoding.DecodeString(enc); err == nil {
			s = strings.TrimSpace(string(b))
		} else if b, err := base64.URLEncoding.DecodeString(enc); err == nil {
			s = strings.TrimSpace(string(b))
		}
	}
	return s
}

func SaveSecret(accountID, secret string) error {
	path, err := secretPath(accountID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(normalizeSecret(secret)), 0600)
}

func GetSecret(accountID string) (string, error) {
	path, err := secretPath(accountID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("secret get %s: %w", accountID, err)
	}
	s := normalizeSecret(string(data))
	if s == "" {
		return "", fmt.Errorf("empty secret for %s", accountID)
	}
	// 若读到 keyring 编码，写回明文，方便服务器长期使用
	if strings.HasPrefix(strings.TrimSpace(string(data)), keyringPrefix) {
		_ = os.WriteFile(path, []byte(s), 0600)
	}
	return s, nil
}

func DeleteSecret(accountID string) error {
	path, err := secretPath(accountID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasSecret 判断账号是否已有可用凭证文件
func HasSecret(accountID string) bool {
	_, err := GetSecret(accountID)
	return err == nil
}
