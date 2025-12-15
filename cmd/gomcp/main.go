package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"gomcp-pilot/internal/app"
	"gomcp-pilot/internal/config"
)

func main() {
	cfgPath := config.DefaultPath()

	rootCmd := &cobra.Command{
		Use:   "gomcp",
		Short: "gomcp-pilot: 本地 MCP 守护进程",
	}
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", cfgPath, "配置文件路径（默认 ~/.config/gomcp/config.yaml）")

	rootCmd.AddCommand(startCmd(&cfgPath))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func startCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "启动 gomcp-pilot 守护进程",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			runtime, err := app.New(cfg)
			if err != nil {
				return fmt.Errorf("init app: %w", err)
			}

			return runtime.Run(ctx)
		},
	}
}
