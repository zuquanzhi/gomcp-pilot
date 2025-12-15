package interceptor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gomcp-pilot/internal/config"
	"gomcp-pilot/internal/tui"
)

type Call struct {
	Tool   string `json:"tool"`
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
	Body   string `json:"body,omitempty"`
}

type Decision struct {
	Allowed bool
	Reason  string
}

type Interceptor struct {
	cfg *config.Config
	ui  *tui.UI
}

func New(cfg *config.Config, ui *tui.UI) *Interceptor {
	return &Interceptor{cfg: cfg, ui: ui}
}

func (i *Interceptor) Evaluate(ctx context.Context, c Call) Decision {
	if i.autoApprove(c.Tool) {
		return Decision{Allowed: true, Reason: "auto_approve=true"}
	}

	if !isRisky(c.Action) {
		return Decision{Allowed: true, Reason: "safe operation"}
	}

	i.ui.Log(fmt.Sprintf("[intercept] risky %s on %s", c.Action, c.Target))
	allowed := promptConfirm(ctx, c)
	if !allowed {
		return Decision{Allowed: false, Reason: "user denied"}
	}
	return Decision{Allowed: true, Reason: "user approved"}
}

func (i *Interceptor) autoApprove(tool string) bool {
	for _, ups := range i.cfg.Upstreams {
		if ups.Name == tool {
			return ups.AutoApprove
		}
	}
	return false
}

func isRisky(action string) bool {
	action = strings.ToLower(action)
	return strings.Contains(action, "write") || strings.Contains(action, "exec")
}

func promptConfirm(ctx context.Context, c Call) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("[?] Risky call detected\nTool: %s\nAction: %s\nTarget: %s\nAllow? [y/N]: ", c.Tool, c.Action, c.Target)

	type result struct {
		ok  bool
		err error
	}
	ch := make(chan result, 1)

	go func() {
		input, err := reader.ReadString('\n')
		if err != nil {
			ch <- result{ok: false, err: err}
			return
		}
		input = strings.TrimSpace(strings.ToLower(input))
		ch <- result{ok: input == "y" || input == "yes", err: nil}
	}()

	select {
	case <-ctx.Done():
		return false
	case res := <-ch:
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "prompt error: %v\n", res.err)
			return false
		}
		return res.ok
	case <-time.After(60 * time.Second):
		fmt.Fprintln(os.Stderr, "prompt timeout: auto deny")
		return false
	}
}
