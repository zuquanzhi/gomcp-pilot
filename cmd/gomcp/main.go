package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gomcp-pilot/internal/app"
	"gomcp-pilot/internal/config"
)

func main() {
	cfgPath := config.DefaultPath()

	root := &cobra.Command{
		Use:   "gomcp",
		Short: "gomcp-pilot: MCP gateway with stdio upstreams",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cfgPath)
		},
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", cfgPath, "path to config file")

	root.AddCommand(startCmd(&cfgPath))
	root.AddCommand(serveCmd(&cfgPath))
	root.AddCommand(mcpCmd(&cfgPath))

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func startCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "start", // Keep 'start' as TUI for backward compat, or change to 'tui'?
		Short: "Start the gomcp-pilot gateway with TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(*cfgPath)
		},
	}
}

func serveCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the gomcp-pilot gateway in headless mode (no TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeadless(*cfgPath)
		},
	}
}

func mcpCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "stdio",
		Short: "Run as an MCP server over stdio (for MCP-compatible clients)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			ctx, cancel := app.WithSignals()
			defer cancel()
			return app.RunMCP(ctx, cfg)
		},
	}
}

func runTUI(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	ctx, cancel := app.WithSignals()
	defer cancel()

	return app.RunTUI(ctx, cfg)
}

func runHeadless(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	ctx, cancel := app.WithSignals()
	defer cancel()

	return app.RunHeadless(ctx, cfg)
}
