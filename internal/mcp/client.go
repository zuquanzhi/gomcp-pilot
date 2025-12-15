package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Client 维护与单个 MCP 子进程的 stdio 通道。
// 协议层目前仅做“写一行 JSON，读一行响应”占位实现。
type Client struct {
	Name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// Start 启动子进程并建立管道。
func Start(ctx context.Context, name, command string, args []string, workdir string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	return &Client{
		Name:   name,
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Call 将 payload 序列化为一行 JSON 写入子进程 stdin，并读取一行响应。
// 注意：MCP 协议对 framing 有更严格要求，此处为占位实现。
func (c *Client) Call(ctx context.Context, payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	if _, err := c.stdin.Write(append(body, '\n')); err != nil {
		return "", fmt.Errorf("write stdin: %w", err)
	}

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := c.stdout.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.err != nil && res.err != io.EOF {
			return "", fmt.Errorf("read stdout: %w", res.err)
		}
		return res.line, nil
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("upstream timeout")
	}
}

// Stop 终止子进程。
func (c *Client) Stop() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

// PID 返回子进程的 PID，若未启动则为 0。
func (c *Client) PID() int {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}
