# gomcp-pilot: 本地 MCP 网关

gomcp-pilot 是一个实现了 Model Context Protocol (MCP) 的本地网关服务。它通过标准输入输出 (stdio) 编排管理多个下游 MCP 服务器，并提供统一的 HTTP 接口供上游 AI Agent 调用。

该项目包含一个基于终端的用户界面 (TUI)，用于实时监控服务状态、查看日志及进行安全拦截。

## 核心功能

*   **安全拦截 (Security Interception)**: 支持对特定上游服务的工具调用进行拦截。当配置为需审批时，TUI 会弹出模态框等待人工确认，防止非预期的高风险操作。
*   **多语言支持 (Polyglot Support)**: 可同时管理 Go、Python、Node.js 等多种语言编写的 MCP 服务器进程。
*   **可视化监控**: 提供基于 Bubbletea 的 TUI 面板，展示上游服务的运行状态、调用统计及实时标准输出/错误日志。
*   **审计与日志**:
    *   SQLite 本地数据库 (`~/.gomcp/audit.db`): 结构化记录每一次工具调用的请求与响应。
    *   应用日志 (`~/.gomcp/gomcp.log`): 记录网关自身的运行日志。
*   **稳定性管理**: 内置进程健康管理，支持启动超时控制与错误捕获。

## 快速开始

### 环境依赖
*   **Go** 1.23+
*   **Python 3** (若需运行 Python 示例/客户端)
*   **Node.js** (若需运行 Node.js 示例/官方文件系统服务)

### 安装

1.  克隆代码库并安装相关语言依赖：
    ```bash
    # 安装 Node.js 依赖 (用于 filesystem server)
    ./scripts/install_deps.sh
    ```

2.  编译本地测试服务 (可选，推荐):
    ```bash
    mkdir -p bin
    go build -o bin/local-utils ./scripts/servers/local_mcp_server.go
    ```

### 启动服务
运行主程序启动网关及 TUI：

```bash
go run ./cmd/gomcp start
```

### 客户端测试
项目提供了一个 Python 脚本用于测试与网关的交互。该脚本会自动发现所有注册工具并与兼容 OpenAI 接口的模型进行对话。

```bash
# 配置环境变量
export OPENAI_API_KEY="sk-..."
export OPENAI_BASE_URL="https://api.deepseek.com" # 默认为 DeepSeek

# 运行交互式客户端
python3 scripts/ai_agent.py
```

## 配置说明

配置文件默认位于 `config.yaml`。该文件定义了网关监听端口及上游服务列表。

```yaml
port: 8080
auth_token: "TEST" # HTTP API 鉴权 Token

upstreams:
  # 示例：官方 Node.js 文件系统服务
  - name: "filesystem"
    command: "node"
    args: ["./local_servers/node_modules/@modelcontextprotocol/server-filesystem/dist/index.js", "."]
    auto_approve: false  # 设置为 false 时，调用需在 TUI 中人工确认

  # 示例：Python 实现的加密工具服务
  - name: "crypto-py"
    command: "python3"
    args: ["scripts/servers/crypto_server.py"]
    auto_approve: true   # 设置为 true 时，自动放行调用

  # 示例：Node.js 实现的数学服务
  - name: "math-js"
    command: "node"
    args: ["scripts/servers/math_server.js"]
    auto_approve: true
```

## 项目结构

*   `cmd/gomcp`: 程序入口
*   `internal/app`: 应用生命周期管理
*   `internal/mcpbridge`: MCP 协议桥接与转发逻辑
*   `internal/process`: 子进程生命周期管理
*   `internal/tui`: 终端用户界面实现
*   `internal/store`: SQLite 审计存储
*   `scripts/servers`: 多语言 MCP 服务器参考实现

## API 接口

网关提供以下 HTTP 接口：

*   `GET /tools/list`: 获取工具列表
    *   返回所有已注册上游服务的工具定义。

*   `POST /tools/call`: 工具调用
    *   Header: `Authorization: Bearer <token>`
    *   Body:
        ```json
        {
          "upstream": "filesystem",
          "tool": "list_directory",
          "arguments": { "path": "." }
        }
        ```
