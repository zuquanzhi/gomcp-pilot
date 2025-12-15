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
			return run(cfgPath)
		},
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", cfgPath, "path to config file")

	root.AddCommand(startCmd(&cfgPath))

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func startCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the gomcp-pilot gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(*cfgPath)
		},
	}
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	ctx, cancel := app.WithSignals()
	defer cancel()

	return app.Run(ctx, cfg)
}
