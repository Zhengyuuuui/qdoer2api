package account

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 文件存储密钥（服务器友好，不依赖 macOS Keychain）
// 路径: ~/.qoder2api/secrets/<id>.token

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

func SaveSecret(accountID, secret string) error {
	path, err := secretPath(accountID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(secret)), 0600)
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
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "", fmt.Errorf("empty secret for %s", accountID)
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
