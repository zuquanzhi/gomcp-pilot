# gomcp-pilot

本地运行的 MCP 网关：通过 stdio 拉起下游 MCP 服务器，聚合工具，经由 HTTP 暴露给上游客户端。当前实现基于 `github.com/mark3labs/mcp-go` 的 stdio 客户端。

## 快速开始
1) 配置 `config.example.yaml`（复制到目标路径），确认 `auth_token`、`port`、以及 filesystem upstream 的允许目录（macOS `/tmp` 请用 `/private/tmp`）。
2) 拉取依赖（需要网络）：`go mod tidy`
3) 启动：`go run ./cmd/gomcp --config config.example.yaml`
4) 健康检查：`curl -H "Authorization: Bearer <token>" http://localhost:8080/health`
5) 列工具：`curl -H "Authorization: Bearer <token>" http://localhost:8080/tools/list`
6) 调用示例：
   ```bash
   curl -X POST -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
     -d '{"upstream":"filesystem","tool":"list_directory","arguments":{"path":"/your/allowed/root"}}' \
     http://localhost:8080/tools/call
   ```

## ToDo
**已完成**
- 使用 `mcp-go` stdio 客户端握手并缓存工具列表
- HTTP API：`/health`、`/tools/list`、`/tools/call`，可选 Bearer Auth
- YAML 配置示例与 CLI 启动（Cobra）
- 简易 smoke client 脚本

**待完成**
- 依赖下载与 `go.sum`（当前网络受限，需在线补充）
- 风险拦截/TUI 审批链（原占位实现未接入新架构）
- 日志与持久化审计（SQLite store）
- 多 upstream 运行状态可视化与错误回收
- 自动化测试与 CI 脚本
