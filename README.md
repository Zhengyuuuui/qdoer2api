# qoder2api

轻量级 [Qoder](https://qoder.ai) → OpenAI / Claude / Codex 兼容 API 网关。

仅包含 **Web 控制台 + Bridge**，无 macOS 桌面 GUI、无 Wails、无系统托盘，适合 Linux 服务器与本地轻量部署。

本项目基于 [wangtufly/QCCG](https://github.com/wangtufly/QCCG) **二次开发**，抽取 Bridge / 账号 / 签名核心逻辑，重做成可服务器部署的精简版本。

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-见原项目-blue)](https://github.com/wangtufly/QCCG/blob/main/LICENSE)

---

## 功能

- 将 Qoder 账号转为本地兼容 API，供 **NewAPI**、**OpenCode**、**Claude Code**、**Codex** 等客户端使用
- 多账号管理：支持 **OAuth** 与 **PAT**
- 账号额度展示
- 一键复制 NewAPI 渠道配置（Base URL + API Key）
- 数据与密钥落盘，适合服务器常驻与保活

## 端口

| 服务 | 默认地址 | 说明 |
|------|----------|------|
| **Web 控制台** | `http://127.0.0.1:3588` | 账号登录、额度、密钥配置 |
| **Bridge API** | `http://127.0.0.1:8963` | OpenAI / Claude / Codex 兼容接口 |

## 快速开始

### 1. 编译

```bash
git clone <你的仓库地址>/qoder2api.git
cd qoder2api
go build -ldflags="-s -w" -o qoder2api .
```

Linux 交叉编译示例：

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o qoder2api .
```

### 2. 启动（控制台 + 桥一体）

一条命令同时启动 **网页控制台** 与 **Bridge 服务框架**：

```bash
./qoder2api --web-port=3588 --bridge-port=8963
```

可选参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `--web-port` | `3588` | Web 控制台端口 |
| `--bridge-port` | `8963` | Bridge API 端口 |
| `--bind` | `0.0.0.0` | 控制台监听地址（服务器可公网访问控制台） |

启动成功后日志类似：

```text
qoder2api console: http://0.0.0.0:3588
bridge target port: 8963
```

### 3. 打开网页控制台

浏览器访问：

```text
http://127.0.0.1:3588
```

服务器部署时替换为：

```text
http://<服务器IP>:3588
```

在控制台中：

1. 选择 Region（`global` / `cn`）
2. 点击 **OAuth 登录**（或填写 PAT 添加）
3. 授权成功后账号会写入本地，并自动 **激活**
4. 激活后 **Bridge 自动监听 8963**

### 4. 启动 / 确认 Bridge

- **自动**：激活账号后会自动 `startBridge`
- **手动**：控制台可调用启动逻辑；也可用 API：

```bash
# 查看状态
curl http://127.0.0.1:3588/api/status

# 确认 Bridge 模型列表
curl http://127.0.0.1:8963/v1/models
```

若返回模型 JSON，说明桥已正常。

---

## 使用方法

### Bridge 端点

| 端点 | 兼容格式 |
|------|----------|
| `POST /v1/chat/completions` | OpenAI Chat |
| `POST /v1/messages` | Anthropic Claude |
| `GET  /v1/models` | 模型列表 |
| `POST /v1/responses` | OpenAI Responses (Codex) |

### 默认接入信息

| 字段 | 值 |
|------|-----|
| Base URL (OpenAI) | `http://127.0.0.1:8963/v1` |
| API Key | `qccg`（可在控制台修改） |
| 推荐模型 | `auto` |

控制台 **「NewAPI / 中转站接入」** 区域可复制完整配置。

### 接入 NewAPI

1. 打开 qoder2api 控制台 → 复制 Base URL 与 API Key  
2. NewAPI 新建渠道：
   - 类型：`OpenAI`
   - Base URL：`http://<host>:8963/v1`
   - Key：控制台中的密钥  
3. 模型填 Qoder 侧 ID，例如：`auto`、`ultimate`、`qmodel_latest`、`kmodel_latest` 等  

### 接入 OpenCode

在 `~/.config/opencode/opencode.json` 中增加 provider：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "qoder2api": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Qoder2API",
      "options": {
        "baseURL": "http://127.0.0.1:8963/v1",
        "apiKey": "qccg"
      },
      "models": {
        "auto": { "name": "Qoder Auto" },
        "ultimate": { "name": "Ultimate" },
        "qmodel_latest": { "name": "Qwen Max" }
      }
    }
  },
  "model": "qoder2api/auto"
}
```

### 接入 Claude Code

```bash
# 示例：环境变量方式（也可写 ~/.claude/settings.json）
export ANTHROPIC_BASE_URL="http://127.0.0.1:8963"
export ANTHROPIC_AUTH_TOKEN="qccg"
export ANTHROPIC_MODEL="auto"
claude
```

### 命令行快速自测

```bash
curl http://127.0.0.1:8963/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer qccg" \
  -d '{"model":"auto","messages":[{"role":"user","content":"你好"}]}'
```

---

## 数据目录

```text
~/.qoder2api/
├── accounts/          # 账号元数据 JSON
├── secrets/           # OAuth/PAT 凭证（权限 0600）
├── settings.json      # 端口、bridge_token 等
└── logs/              # 运行日志
```

服务器上请保证进程用户对 `$HOME/.qoder2api` 可写；用 Docker 时挂载数据卷到 `/data`（`HOME=/data`）。

---

## Docker 部署

```bash
docker build -t qoder2api .
docker run -d --name qoder2api \
  -p 3588:3588 -p 8963:8963 \
  -v qoder2api-data:/data \
  -e HOME=/data \
  qoder2api
```

然后访问：`http://<服务器IP>:3588` 完成 OAuth 登录。

## systemd 部署

```bash
sudo mkdir -p /opt/qoder2api
sudo cp qoder2api /opt/qoder2api/
sudo cp qoder2api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now qoder2api
```

---

## 项目结构

```text
qoder2api/
├── main.go              # 入口：Web + API
├── service.go           # 账号 / Bridge 业务
├── account/             # 账号、OAuth、设置、密钥文件存储
├── internal/
│   ├── bridge/          # OpenAI / Claude / Codex 兼容层
│   └── cosy/            # Qoder 签名与会话
├── logger/
├── webconsole/          # 轻量 HTML 控制台
├── baseprompt.json
├── Dockerfile
└── qoder2api.service
```

---

## 致谢

- **[QCCG](https://github.com/wangtufly/QCCG)**（[wangtufly](https://github.com/wangtufly)）  
  本项目在 QCCG 之上进行二次开发：复用 / 精简了 Bridge、账号体系、OAuth 与 Qoder 协议相关实现，并改为无 GUI 的服务器友好形态。  
  **感谢原作者的开源工作。** 若你需要完整的 macOS 桌面端体验，请优先使用官方 QCCG。

- 上游思路与生态亦受益于 Qoder 社区及相关逆向/兼容项目（见 QCCG 仓库 README 中的鸣谢）。

---

## 免责声明

本项目仅供学习与自用。请遵守 [Qoder](https://qoder.ai) 服务条款与当地法律法规；账号与 API 使用风险自负。

---

## License

二次开发基于 [QCCG](https://github.com/wangtufly/QCCG) 的开源协议；请同时遵守原项目 [LICENSE](https://github.com/wangtufly/QCCG/blob/main/LICENSE) 的要求。
