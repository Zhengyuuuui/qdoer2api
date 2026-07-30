package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const storeDir = ".qoder2api/accounts"
const settingsFile = ".qoder2api/settings.json"

var mu sync.Mutex

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, storeDir)
	return d, os.MkdirAll(d, 0700)
}

func List() ([]Account, error) {
	mu.Lock()
	defer mu.Unlock()
	d, err := dir()
	if err != nil {
		return nil, err
	}
	return listUnlocked(d)
}

func Save(a *Account) error {
	mu.Lock()
	defer mu.Unlock()
	d, err := dir()
	if err != nil {
		return err
	}
	if a.ID == "" {
		a.ID = SanitizeID(a.Email + a.Name + fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	return saveUnlocked(d, a)
}

func Delete(id string) error {
	mu.Lock()
	defer mu.Unlock()
	d, err := dir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(d, SanitizeID(id)+".json"))
}

func SetActive(id string) error {
	mu.Lock()
	defer mu.Unlock()
	d, err := dir()
	if err != nil {
		return err
	}
	accounts, err := listUnlocked(d)
	if err != nil {
		return err
	}
	for i := range accounts {
		accounts[i].Active = accounts[i].ID == id
		if err := saveUnlocked(d, &accounts[i]); err != nil {
			return err
		}
	}
	return nil
}

func GetActive() (*Account, error) {
	accounts, err := List()
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		if accounts[i].Active {
			return &accounts[i], nil
		}
	}
	return nil, nil
}

// Get 根据 ID 获取账号
func Get(id string) (*Account, error) {
	accounts, err := List()
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		if accounts[i].ID == id {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("account not found: %s", id)
}

func listUnlocked(d string) ([]Account, error) {
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(d, e.Name()))
		if err != nil {
			continue
		}
		var a Account
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].SortOrder < accounts[j].SortOrder
	})
	return accounts, nil
}

func saveUnlocked(d string, a *Account) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, a.ID+".json"), data, 0600)
}

func Reorder(ids []string) error {
	mu.Lock()
	defer mu.Unlock()
	d, err := dir()
	if err != nil {
		return err
	}
	accounts, err := listUnlocked(d)
	if err != nil {
		return err
	}
	orderMap := make(map[string]int)
	for i, id := range ids {
		orderMap[id] = i
	}
	for i := range accounts {
		if order, ok := orderMap[accounts[i].ID]; ok {
			accounts[i].SortOrder = order
		}
		if err := saveUnlocked(d, &accounts[i]); err != nil {
			return err
		}
	}
	return nil
}

func SanitizeID(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = fmt.Sprintf("acct%d", time.Now().UnixNano())
	}
	return s
}

func LoadSettings() (*Settings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, settingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		// 返回默认设置
		return &Settings{
			Port:      8963,
			AutoStart: false,
			LogLevel:  "info",
		}, nil
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveSettings(s *Settings) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Join(home, settingsFile))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, settingsFile), data, 0600)
}
