# gomcp-pilot (skeleton)

本仓库包含基于文档落地的工程级骨架，方便继续迭代为本地 MCP 守护进程。

## 目录结构
- `cmd/gomcp/`: Cobra CLI 入口（`gomcp start`）。
- `internal/config`: viper 加载 config.yaml。
- `internal/process`: 子进程编排（启动/销毁 upstream）。
- `internal/interceptor`: Risky 调用拦截与人工确认。
- `internal/server`: HTTP/SSE 服务（/health, /tools/list, /tools/call, /events）。
- `internal/tui`: Bubbletea 事件视图。
- `internal/store`: SQLite 审计占位实现。

## 快速开始
1. 安装 Go 1.22+。
2. 在可联网环境下执行 `go mod tidy` 拉取依赖（cobra/bubbletea/sqlite 等）。
3. 准备配置文件（默认 `~/.config/gomcp/config.yaml`，或直接使用仓库里的 `config.example.yaml`）：
   ```yaml
   port: 8080
   auth_token: "demo-token"
   upstreams:
     - name: "project-files"
       command: "npx"
       args: ["-y", "@modelcontextprotocol/server-filesystem", "/Users/me/Code"]
       auto_approve: false
   ```
4. 运行 `go run ./cmd/gomcp start --config ./config.example.yaml`。

## TODO
- MCP 协议：完善 stdin/stdout framing，支持正式的 MCP RPC 循环（当前按行写读占位实现）。
- 审计存储：为 SQLite 设计 schema 并落库（`internal/store` 仍为空壳）。
- 上游 env/secret：在配置中支持 env 注入，减少依赖 `export`。
- UI：在 TUI 中展示请求体/响应 JSON，以及 upstream 运行状态。
- 包发布：提供预编译二进制与更完善的安装脚本。
