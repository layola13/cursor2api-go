# Cursor2API

[English](README_EN.md) | 简体中文

一个将 Cursor Web 转换为 OpenAI 兼容 API 的服务。

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## ✨ 特性

- 🔄 **API 兼容**: 完全兼容 OpenAI API 格式
- ⚡ **高性能**: 低延迟响应
- 🔐 **安全认证**: 支持 API Key 认证
- 🌐 **多模型支持**: 支持多种 AI 模型
- 🛡️ **错误处理**: 完善的错误处理机制
- 📊 **健康检查**: 内置健康检查接口

## ✨ 功能特性

- ✅ 完全兼容 OpenAI API 格式
- ✅ 支持流式和非流式响应
- ✅ 高性能 Go 语言实现
- ✅ 自动处理 Cursor Web 认证
- ✅ 简洁的 Web 界面

## 🤖 支持的模型

- **Anthropic Claude**: claude-sonnet-4.6

## 🚀 快速开始

### 环境要求

- Go 1.24+
- Node.js 18+ (用于 JavaScript 执行)

### 本地运行方式

#### 方法一：直接运行（推荐用于开发）

**Linux/macOS**:
```bash
git clone https://github.com/libaxuan/cursor2api-go.git
cd cursor2api-go
chmod +x start.sh
./start.sh
```

**Windows**:
```batch
# 双击运行或在 cmd 中执行
start-go.bat

# 或在 Git Bash / Windows Terminal 中
./start-go-utf8.bat
```

#### 方法二：手动编译运行

```bash
# 克隆项目
git clone https://github.com/libaxuan/cursor2api-go.git
cd cursor2api-go

# 下载依赖
go mod tidy

# 编译
go build -o cursor2api-go

# 运行
./cursor2api-go
```

#### 方法三：使用 go run

```bash
git clone https://github.com/libaxuan/cursor2api-go.git
cd cursor2api-go
go run main.go
```

服务将在 `http://localhost:8002` 启动

## 🚀 服务器部署方式

### Docker 部署

1. **构建镜像**:
```bash
# 构建镜像
docker build -t cursor2api-go .
```

2. **运行容器**:
```bash
# 运行容器（推荐）
docker run -d \
  --name cursor2api-go \
  --restart unless-stopped \
  -p 8002:8002 \
  -e API_KEY=your-secret-key \
  -e DEBUG=false \
  cursor2api-go

# 或者使用默认配置运行
docker run -d --name cursor2api-go --restart unless-stopped -p 8002:8002 cursor2api-go
```

### Docker Compose 部署（推荐用于生产环境）

1. **使用 docker-compose.yml**:
```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 查看日志
docker-compose logs -f
```

2. **自定义配置**:
修改 `docker-compose.yml` 文件中的环境变量以满足您的需求：
- 修改 `API_KEY` 为安全的密钥
- 根据需要调整 `MODELS`、`TIMEOUT` 等配置
- 更改暴露的端口

### 系统服务部署（Linux）

1. **编译并移动二进制文件**:
```bash
go build -o cursor2api-go
sudo mv cursor2api-go /usr/local/bin/
sudo chmod +x /usr/local/bin/cursor2api-go
```

2. **创建系统服务文件** `/etc/systemd/system/cursor2api-go.service`:
```ini
[Unit]
Description=Cursor2API Service
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/home/your-user/cursor2api-go
ExecStart=/usr/local/bin/cursor2api-go
Restart=always
Environment=API_KEY=your-secret-key
Environment=PORT=8002

[Install]
WantedBy=multi-user.target
```

3. **启动服务**:
```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启用开机自启
sudo systemctl enable cursor2api-go

# 启动服务
sudo systemctl start cursor2api-go

# 查看状态
sudo systemctl status cursor2api-go
```

## 📡 API 使用

### 获取模型列表

```bash
curl -H "Authorization: Bearer 0000" http://localhost:8002/v1/models
```

### 非流式聊天

```bash
curl -X POST http://localhost:8002/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 0000" \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": false
  }'
```

### 流式聊天

```bash
curl -X POST http://localhost:8002/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 0000" \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### 在第三方应用中使用

在任何支持自定义 OpenAI API 的应用中（如 ChatGPT Next Web、Lobe Chat 等）：

1. **API 地址**: `http://localhost:8002`
2. **API 密钥**: `0000`（或自定义）
3. **模型**: 选择支持的模型之一

## 🧩 XML 工具桥接改造记录（2026-03）

本次改造的目标是：在**不修改客户端 OpenAI function/tool 请求格式**的前提下，服务端拦截并桥接为 XML 协议与 Cursor 对话，最后再转换回标准 OpenAI 响应，让客户端无感。

### 改造步骤与过程

1. **请求侧保持 OpenAI 标准**
- 客户端继续发送 `tools` / `tool_choice` / `function_call`，不做改动。

2. **服务端自动注入 XML 桥接提示词**
- 检测到工具调用请求后，自动注入 Roo 风格 XML 规则，要求模型以 XML 形式调用工具（支持参数子标签、CDATA、多工具 `<tool_calls>` 包裹等）。
- 同时把历史 `assistant.tool_calls`、`tool`/`function` 消息规范化为可供模型理解的 XML 文本上下文，避免信息丢失。

3. **失败重试（不把失败结果返回客户端）**
- 当模型首轮未使用工具时，服务端注入纠正提示（`[ERROR] You did not use a tool...`）并重试，最多 2 次。
- 对 `stream=true` 请求，首轮失败内容在服务端缓存，不提前下发给客户端。

4. **响应回转为标准 OpenAI 格式**
- 非流式：将 XML 工具调用解析为 `choices[0].message.tool_calls`，`finish_reason="tool_calls"`。
- 流式：将 XML 工具调用转换为标准 SSE `delta.tool_calls`，不再把 XML 原文返回给客户端。
- 若模型在 XML 之外还输出普通说明文本，会保留并返回：
  - 非流式：`message.content`
  - 流式：`delta.content`

### 当前行为说明

- **非流式工具调用**：返回 OpenAI 标准 `tool_calls`，可附带文本说明。
- **流式工具调用**：返回 OpenAI 标准增量 `delta.tool_calls`（无 XML 标签泄漏），可附带 `delta.content`。
- **普通对话（无 tools）**：保持原有文本对话流程，不走 XML 桥接。

### 关键代码位置

- `models/tool_bridge.go`：
  - `BuildToolCallBridgePrompt`
  - `NormalizeMessagesForToolBridge`
  - `ExtractXMLToolCalls`
  - `ExtractNonToolTextFromXMLContent`
- `services/cursor.go`：
  - 在发往 Cursor 前启用桥接消息与系统提示注入
- `handlers/handler.go`：
  - `handleNonStreamToolBridgeWithRetry`
  - `handleStreamToolBridgeWithRetry`
  - `streamBufferedToolCallResponse`
- `models/models.go`：
  - 新增流式 `delta.tool_calls` 数据结构
  - `NewChatCompletionToolCallResponse` 支持 `tool_calls + content`

### 验证方式（示例）

1. **Go 单元测试**
```bash
go test ./...
```

2. **流式验证（检查返回是否为标准 tool_calls）**
```bash
curl -sN http://127.0.0.1:8002/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 0000" \
  -d '{
    "model":"claude-sonnet-4.6",
    "stream":true,
    "tool_choice":"required",
    "messages":[{"role":"user","content":"用c++写个简单的排序,保存到本地，然后执行编译测试"}],
    "tools":[
      {"type":"function","function":{"name":"write_file","parameters":{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}}},
      {"type":"function","function":{"name":"run_command","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}
    ]
  }'
```
- 期望：SSE chunk 出现 `delta.tool_calls`，不出现 `<tool_calls>` XML 原文。

3. **TypeScript 客户端回放测试**
- 测试脚本：`ts-tool-test/tool-call-loop.ts`
- 该脚本会模拟 OpenAI 客户端工具循环，验证文件写入和编译命令执行。

## ⚙️ 配置说明

### 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `PORT` | `8002` | 服务器端口 |
| `DEBUG` | `false` | 调试模式（启用后显示详细日志和路由信息） |
| `API_KEY` | `0000` | API 认证密钥 |
| `MODELS` | `claude-sonnet-4.6` | 支持的模型列表（逗号分隔） |
| `TIMEOUT` | `60` | 请求超时时间（秒） |

### 调试模式

默认情况下，服务以简洁模式运行。如需启用详细日志：

**方式 1**: 修改 `.env` 文件
```bash
DEBUG=true
```

**方式 2**: 使用环境变量
```bash
DEBUG=true ./cursor2api-go
```

调试模式会显示：
- 详细的 GIN 路由信息
- 每个请求的详细日志
- x-is-human token 信息
- 浏览器指纹配置

### 故障排除

遇到问题？查看 **[故障排除指南](TROUBLESHOOTING.md)** 了解常见问题的解决方案，包括：
- 403 Access Denied 错误
- Token 获取失败
- 连接超时
- Cloudflare 拦截


### Windows 启动脚本说明

项目提供两个 Windows 启动脚本：

- **`start-go.bat`** (推荐): GBK 编码，完美兼容 Windows cmd.exe
- **`start-go-utf8.bat`**: UTF-8 编码，适用于 Git Bash、PowerShell、Windows Terminal

两个脚本功能完全相同，仅显示样式不同。如遇乱码请使用 `start-go.bat`。

## 🧪 开发

### 运行测试

```bash
# 运行现有测试
go test ./...
```

### 构建项目

```bash
# 构建可执行文件
go build -o cursor2api-go

# 交叉编译 (例如 Linux)
GOOS=linux GOARCH=amd64 go build -o cursor2api-go-linux
```

## 📁 项目结构

```
cursor2api-go/
├── main.go              # 主程序入口 (Go 版本)
├── config/              # 配置管理 (Go 版本)
├── handlers/            # HTTP 处理器 (Go 版本)
├── services/            # 业务服务层 (Go 版本)
├── models/              # 数据模型 (Go 版本)
├── utils/               # 工具函数 (Go 版本)
├── middleware/          # 中间件 (Go 版本)
├── jscode/              # JavaScript 代码 (Go 版本)
├── static/              # 静态文件 (Go 版本)
├── start.sh             # Linux/macOS 启动脚本
├── start-go.bat         # Windows 启动脚本 (GBK)
├── start-go-utf8.bat    # Windows 启动脚本 (UTF-8)

└── README.md            # 项目说明
```

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'feat: Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码规范

- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 使用 `go vet` 检查代码
- 提交信息遵循 [Conventional Commits](https://conventionalcommits.org/) 规范

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## ⚠️ 免责声明

本项目仅供学习和研究使用，请勿用于商业用途。使用本项目时请遵守相关服务的使用条款。

---

⭐ 如果这个项目对您有帮助，请给我们一个 Star！
