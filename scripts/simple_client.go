package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// 最小 MCP 网关客户端，变量写死方便直接跑。
// - 网关地址：baseURL
// - 授权 Token：authToken
// - LLM/AI 上游示例：llmBaseURL + llmAPIKey（仅用于演示透传，不会实际请求 AI）
// - 默认调用：依次调用 filesystem / filesystem-alt 读取 /tmp
const (
	baseURL   = "http://localhost:8080"
	authToken = "demo-token"

	// 示例：如果你要把 AI 的 base URL 和 key 当作调用参数透传给 upstream，可在 payload 中使用。
	llmBaseURL = "https://api.deepseek.com/v1"
	llmAPIKey  = "sk-20061e6d98dc4f628f745e164fbbbcc4"
)

func main() {
	// 列出 upstreams
	mustRequest("GET", baseURL+"/tools/list", authToken, nil)

	// 发起两次调用示例
	payloads := []map[string]any{
		{
			"tool":   "filesystem",
			"action": "read_file",
			"target": "/tmp",
			"llm": map[string]string{
				"base_url": llmBaseURL,
				"api_key":  llmAPIKey,
			},
		},
		{
			"tool":   "filesystem-alt",
			"action": "read_file",
			"target": "/tmp",
		},
	}

	for _, payload := range payloads {
		buf, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("marshal payload: %v\n", err)
			os.Exit(1)
		}
		mustRequest("POST", baseURL+"/tools/call", authToken, buf)
	}
}

func mustRequest(method, url, token string, body []byte) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		fmt.Printf("new request: %v\n", err)
		os.Exit(1)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("%s %s -> %d\n", method, url, resp.StatusCode)
	data, _ := io.ReadAll(resp.Body)
	fmt.Println(string(data))
}
