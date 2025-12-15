package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/sashabaranov/go-openai"
)

// --- 配置区域 ---
const (
	DeepSeekAPIKeyEnv  = "DEEPSEEK_API_KEY"
	DeepSeekBaseURLEnv = "DEEPSEEK_BASE_URL"
	GatewayBaseURLEnv  = "GOMCP_BASE_URL" // 可选：如果未来实现了 SSE/HTTP MCP，可使用此地址
	GatewayTokenEnv    = "GOMCP_TOKEN"    // 可选：如果需要自定义 Header，可在 WithHeaderFunc 中使用
	// stdio 方式启动 gomcp（默认）
	StdioCmdEnv  = "GOMCP_STDIN_CMD"  // 默认 "go"
	StdioArgsEnv = "GOMCP_STDIN_ARGS" // 默认 "run ./cmd/gomcp stdio --config config.example.yaml"
)

func main() {
	ctx := context.Background()

	baseURL := os.Getenv(GatewayBaseURLEnv) // 留空时走 stdio
	var mcpClient *client.Client
	var err error

	if baseURL != "" {
		fmt.Printf("正在连接 MCP 网关 (SSE/HTTP): %s ...\n", baseURL)
		mcpClient, err = client.NewSSEMCPClient(baseURL)
		if err != nil {
			log.Fatalf("创建 SSE 客户端失败: %v", err)
		}
	} else {
		// 默认：启动本地 gomcp stdio
		cmd := envWithDefault(StdioCmdEnv, "go")
		argStr := envWithDefault(StdioArgsEnv, "run ./cmd/gomcp stdio --config config.example.yaml")
		args := strings.Fields(argStr)
		fmt.Printf("正在通过 stdio 启动 gomcp: %s %s\n", cmd, strings.Join(args, " "))

		// 如果需要自定义工作目录，可在这里设置 exec.Cmd.Dir
		stdio := transport.NewStdioWithOptions(cmd, nil, args, transport.WithCommandFunc(func(ctx context.Context, c string, env []string, a []string) (*exec.Cmd, error) {
			ecmd := exec.CommandContext(ctx, c, a...)
			ecmd.Env = append(os.Environ(), env...)
			return ecmd, nil
		}))
		mcpClient = client.NewClient(stdio)
	}
	defer mcpClient.Close()

	// 启动传输
	if err := mcpClient.Start(ctx); err != nil {
		log.Fatalf("无法连接/启动 MCP 客户端: %v", err)
	}

	// 2. 初始化 MCP 握手
	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "deepseek-remote-agent", Version: "1.0.0"},
		},
	})
	if err != nil {
		log.Fatalf("MCP 握手失败: %v", err)
	}

	// 3. 获取工具列表
	toolList, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalf("无法获取工具列表: %v", err)
	}
	log.Printf("成功加载 %d 个工具", len(toolList.Tools))

	// 4. 转换工具定义 (MCP -> OpenAI)
	openaiTools := make([]openai.Tool, len(toolList.Tools))
	nameMap := make(map[string]string, len(toolList.Tools)) // safe -> full
	for i, t := range toolList.Tools {
		schemaBytes, _ := json.Marshal(t.InputSchema)
		safeName := sanitizeToolName(t.Name)
		nameMap[safeName] = t.Name
		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        safeName,
				Description: fmt.Sprintf("%s (MCP: %s)", t.Description, t.Name),
				Parameters:  json.RawMessage(schemaBytes),
			},
		}
	}

	// 5. 初始化 LLM
	apiKey := os.Getenv(DeepSeekAPIKeyEnv)
	if apiKey == "" {
		log.Fatalf("环境变量 %s 未设置", DeepSeekAPIKeyEnv)
	}
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = envWithDefault(DeepSeekBaseURLEnv, "https://api.deepseek.com")
	llmClient := openai.NewClientWithConfig(config)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a helpful assistant connecting to a remote MCP Gateway. Use tools when necessary. Use absolute paths for file operations.",
		},
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n--- DeepSeek Remote Agent 已就绪 (输入 'quit' 退出) ---")

	// 6. 对话循环
	for {
		fmt.Print("\nUser: ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "quit" {
			break
		}
		if userInput == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userInput,
		})

		processLLMTurn(ctx, llmClient, mcpClient, &messages, openaiTools, nameMap)
	}
}

func envWithDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// processLLMTurn 逻辑保持不变
func processLLMTurn(
	ctx context.Context,
	llm *openai.Client,
	mcpClient *client.Client,
	messages *[]openai.ChatCompletionMessage,
	tools []openai.Tool,
	nameMap map[string]string,
) {
	fmt.Print("DeepSeek 思考中...")

	resp, err := llm.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "deepseek-chat",
		Messages: *messages,
		Tools:    tools,
	})
	if err != nil {
		fmt.Printf("\nLLM 请求错误: %v\n", err)
		return
	}

	msg := resp.Choices[0].Message
	*messages = append(*messages, msg)
	fmt.Print("\r                  \r")

	if len(msg.ToolCalls) > 0 {
		fmt.Printf("\n[Agent] DeepSeek 请求调用 %d 个工具...\n", len(msg.ToolCalls))

		for _, toolCall := range msg.ToolCalls {
			safeName := toolCall.Function.Name
			toolName, ok := nameMap[safeName]
			if !ok {
				fmt.Printf("  -> 未知工具名（未映射）: %s\n", safeName)
				continue
			}
			argsJSON := toolCall.Function.Arguments

			// 简单的显示优化
			displayArgs := argsJSON
			if len(displayArgs) > 50 {
				displayArgs = displayArgs[:50] + "..."
			}
			fmt.Printf("  -> 调用: %s (%s)\n", toolName, displayArgs)

			var args map[string]interface{}
			_ = json.Unmarshal([]byte(argsJSON), &args)

			// 网络调用网关
			mcpResult, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      toolName,
					Arguments: args,
				},
			})

			var toolOutput string
			if err != nil {
				toolOutput = fmt.Sprintf("Error: %v", err)
			} else {
				resultBytes, _ := json.Marshal(mcpResult)
				toolOutput = string(resultBytes)
			}

			// 截断显示
			if len(toolOutput) > 100 {
				fmt.Printf("  <- 结果: %s...\n", toolOutput[:100])
			} else {
				fmt.Printf("  <- 结果: %s\n", toolOutput)
			}

			*messages = append(*messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    toolOutput,
				ToolCallID: toolCall.ID,
			})
		}

		// 递归调用下一轮
		processLLMTurn(ctx, llm, mcpClient, messages, tools, nameMap)

	} else {
		fmt.Printf("\nDeepSeek: %s\n", msg.Content)
	}
}

// sanitizeToolName converts MCP tool names (may contain '/') to OpenAI-allowed names.
func sanitizeToolName(name string) string {
	return strings.ReplaceAll(name, "/", "__")
}
