package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qoder2api/account"
	"qoder2api/logger"
)

//go:embed all:webconsole
var webAssets embed.FS

//go:embed baseprompt.json
var basePromptRaw []byte

var svc *Service

func main() {
	webPort := flag.Int("web-port", 3588, "web console port")
	bridgePort := flag.Int("bridge-port", 8963, "bridge api port")
	bind := flag.String("bind", "0.0.0.0", "console bind address")
	flag.Parse()

	if home, err := os.UserHomeDir(); err == nil {
		logDir := filepath.Join(home, ".qoder2api", "logs")
		if err := logger.InitFile(logDir); err != nil {
			fmt.Fprintf(os.Stderr, "[logger] init failed: %v\n", err)
		}
	}

	svc = NewService(basePromptRaw)
	svc.LoadRuntimeSettings()
	if *bridgePort > 0 {
		svc.bridgePort = *bridgePort
	}

	// 控制台密码：环境变量 > settings；都没有则自动生成并落盘
	_ = ensureConsolePassword()

	// 启动时尽量自动拉起 Bridge：优先激活账号，否则尝试任意有 secret 的账号
	if err := svc.EnsureBridgeRunning(); err != nil {
		logger.Error("auto start bridge failed: %v", err)
		logger.Info("open console and login/activate an account: http://127.0.0.1:%d", *webPort)
	}

	sub, err := fs.Sub(webAssets, "webconsole")
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	mux := http.NewServeMux()
	// auth (public)
	mux.HandleFunc("/api/auth/login", handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", handleAuthLogout)
	mux.HandleFunc("/api/auth/me", handleAuthMe)

	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/accounts", handleAccounts)
	mux.HandleFunc("/api/accounts/add", handleAddAccount)
	mux.HandleFunc("/api/accounts/delete", handleDeleteAccount)
	mux.HandleFunc("/api/accounts/quota", handleAccountQuota)
	mux.HandleFunc("/api/oauth/start", handleOAuthStart)
	mux.HandleFunc("/api/oauth/wait", handleOAuthWait)
	mux.HandleFunc("/api/oauth/cancel", handleOAuthCancel)
	mux.HandleFunc("/api/active", handleSetActiveAccount)
	mux.HandleFunc("/api/settings", handleGetSettings)
	mux.HandleFunc("/api/settings/save", handleSaveSettings)
	mux.HandleFunc("/api/models", handleListModels)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/cleanup", handleCleanup)
	mux.HandleFunc("/api/bridge/start", handleStartBridge)
	mux.HandleFunc("/api/bridge/stop", handleStopBridge)
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := fmt.Sprintf("%s:%d", *bind, *webPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      withCORS(requireConsoleAuth(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute, // oauth wait may block
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("qoder2api console: http://%s (password protected)", addr)
	logger.Info("bridge target port: %d (API key separate from console password)", svc.bridgePort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(dst)
}

func connectionInfo() map[string]interface{} {
	port := 8963
	token := "qccg"
	if svc != nil {
		if svc.bridgePort > 0 {
			port = svc.bridgePort
		}
		token = svc.EffectiveToken()
	}
	// 控制台展示用：服务器上请改成公网/内网 IP
	host := "127.0.0.1"
	base := fmt.Sprintf("http://%s:%d", host, port)
	return map[string]interface{}{
		"port":            port,
		"api_key":         token,
		"base_url":        base,
		"openai_base_url": base + "/v1",
		"claude_base_url": base,
		"newapi_hint": map[string]string{
			"type":     "openai",
			"base_url": base + "/v1",
			"api_key":  token,
			"model":    "auto",
		},
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, svc.GetStatus())
}

func handleAccounts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, svc.ListAccounts())
}

func handleAddAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		PAT    string `json:"pat"`
		Region string `json:"region"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.PAT = strings.TrimSpace(req.PAT)
	if req.PAT == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("pat is required"))
		return
	}
	acct, err := svc.AddAccountByPAT(req.PAT, req.Region)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := svc.SetActiveAccount(acct.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, acct)
}

func handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := svc.DeleteAccount(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleAccountQuota(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" && r.Method == http.MethodPost {
		var req struct {
			ID string `json:"id"`
		}
		if err := readJSON(r, &req); err == nil {
			id = strings.TrimSpace(req.ID)
		}
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}
	quota, err := svc.GetAccountQuota(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, quota)
}

func handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		Region string `json:"region"`
	}
	_ = readJSON(r, &req)
	session, err := svc.StartOAuthLogin(req.Region)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func handleOAuthWait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		LoginID string `json:"login_id"`
		Active  *bool  `json:"active"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.LoginID = strings.TrimSpace(req.LoginID)
	if req.LoginID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("login_id is required"))
		return
	}
	acct, err := account.WaitLogin(req.LoginID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := account.Save(acct); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	activate := true
	if req.Active != nil {
		activate = *req.Active
	}
	if activate {
		if err := svc.SetActiveAccount(acct.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, acct)
}

func handleOAuthCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		LoginID string `json:"login_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	svc.CancelOAuthLogin(strings.TrimSpace(req.LoginID))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleSetActiveAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := svc.SetActiveAccount(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := svc.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := *settings
	if out.BridgeToken == "" {
		out.BridgeToken = "qccg"
	}
	if out.Port == 0 {
		out.Port = 8963
	}
	// 不回传明文控制台密码，只标记是否已设置
	hasConsolePwd := strings.TrimSpace(out.ConsolePassword) != "" || strings.TrimSpace(os.Getenv("QODER2API_CONSOLE_PASSWORD")) != ""
	out.ConsolePassword = ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"settings":              out,
		"connection":            connectionInfo(),
		"console_password_set":  hasConsolePwd,
	})
}

func handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	var req struct {
		Port            *int    `json:"port"`
		BridgeToken     *string `json:"bridge_token"`
		LogLevel        *string `json:"log_level"`
		AutoStart       *bool   `json:"auto_start"`
		ConsolePassword *string `json:"console_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cur, err := svc.GetSettings()
	if err != nil || cur == nil {
		cur = &account.Settings{Port: 8963, LogLevel: "info"}
	}
	if req.Port != nil {
		if *req.Port < 1 || *req.Port > 65535 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid port"))
			return
		}
		cur.Port = *req.Port
	}
	if req.BridgeToken != nil {
		tok := strings.TrimSpace(*req.BridgeToken)
		if tok == "" || tok == "qccg" {
			cur.BridgeToken = ""
		} else {
			cur.BridgeToken = tok
		}
	}
	if req.LogLevel != nil && *req.LogLevel != "" {
		cur.LogLevel = *req.LogLevel
	}
	if req.AutoStart != nil {
		cur.AutoStart = *req.AutoStart
	}
	if req.ConsolePassword != nil {
		p := strings.TrimSpace(*req.ConsolePassword)
		if p != "" {
			cur.ConsolePassword = p
		}
	}
	if err := svc.SaveSettings(cur); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	safe := *cur
	safe.ConsolePassword = ""
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"connection": connectionInfo(),
		"settings":   safe,
		"note":       "已保存；控制台密码立即生效，Bridge 端口变更需重启 Bridge",
	})
}

func handleListModels(w http.ResponseWriter, r *http.Request) {
	models, err := svc.ListQoderModels()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, models)
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, svc.GetLogs(100))
}

func handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	if err := svc.CleanupAllData(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleStartBridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	if err := svc.StartBridge(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func handleStopBridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST required"))
		return
	}
	if err := svc.StopBridge(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
