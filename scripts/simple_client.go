package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// A minimal HTTP client for manual verification of the gateway.
func main() {
	baseURL := "http://localhost:8080"
	authToken := "replace-with-a-strong-secret"

	get(baseURL+"/health", authToken, nil)
	get(baseURL+"/tools/list", authToken, nil)

	callBody := map[string]any{
		"upstream": "filesystem",
		"tool":     "list_files",
		"arguments": map[string]any{
			"path": "/tmp",
		},
	}
	post(baseURL+"/tools/call", authToken, callBody)
}

func get(url, token string, body any) {
	doRequest(http.MethodGet, url, token, body)
}

func post(url, token string, body any) {
	doRequest(http.MethodPost, url, token, body)
}

func doRequest(method, url, token string, body any) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
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
