package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Smoke test: drive the gomcp HTTP gateway as a client, calling any upstream/tool pair.
// Defaults target filesystem/list_directory so it works out-of-the-box; set envs to call DeepSeek (or other) tools.
//
// Env vars:
// - GOMCP_BASE_URL   (default http://localhost:8080)
// - GOMCP_TOKEN      (Bearer token; must match auth_token)
// - GOMCP_UPSTREAM   (default "filesystem")
// - GOMCP_TOOL       (default "list_directory")
// - GOMCP_PATH       (default "/Users/zuquanzhi/test", used when tool expects a path)
// - GOMCP_ARGS_JSON  (optional raw JSON to override arguments entirely)
//
// DeepSeek-specific helpers (used when upstream/tool expect these fields):
// - DEEPSEEK_API_KEY (if set and tool == "chat", will be injected)
// - DEEPSEEK_MODEL   (default "deepseek-chat")
// - DEEPSEEK_PROMPT  (default "用 1 句话介绍 gomcp-pilot 是什么")
func main() {
	baseURL := env("GOMCP_BASE_URL", "http://localhost:8080")
	token := env("GOMCP_TOKEN", "")
	upstream := env("GOMCP_UPSTREAM", "filesystem")
	tool := env("GOMCP_TOOL", "list_directory")

	// 1) list tools to confirm connectivity
	doRequest(http.MethodGet, baseURL+"/tools/list", token, nil)

	// 2) build arguments
	args := buildArgs(upstream, tool)

	// 3) call the tool
	body := map[string]any{
		"upstream":  upstream,
		"tool":      tool,
		"arguments": args,
	}
	doRequest(http.MethodPost, baseURL+"/tools/call", token, body)
}

func buildArgs(upstream, tool string) any {
	// If user provided raw JSON, honor it.
	if raw := os.Getenv("GOMCP_ARGS_JSON"); raw != "" {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			fmt.Fprintf(os.Stderr, "invalid GOMCP_ARGS_JSON: %v\n", err)
			os.Exit(1)
		}
		return v
	}

	// Defaults for filesystem list_directory
	if upstream == "filesystem" && tool == "list_directory" {
		path := env("GOMCP_PATH", "/Users/zuquanzhi/test")
		return map[string]any{"path": path}
	}

	// DeepSeek chat example
	if tool == "chat" {
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		model := env("DEEPSEEK_MODEL", "deepseek-chat")
		prompt := env("DEEPSEEK_PROMPT", "用 1 句话介绍 gomcp-pilot 是什么")
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "DEEPSEEK_API_KEY is required for chat tool")
			os.Exit(1)
		}
		return map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"api_key": apiKey,
		}
	}

	// Fallback: empty args
	return map[string]any{}
}

func doRequest(method, url, token string, body any) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			fmt.Printf("marshal payload: %v\n", err)
			os.Exit(1)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
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

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("%s %s -> %d\n", method, url, resp.StatusCode)
	data, _ := io.ReadAll(resp.Body)
	fmt.Println(string(data))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
