package main

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Config holds CLI flag values
type Config struct {
	Address    string
	NoColor    bool
	OnlyErrors bool
	Search     string
	Filter     string
	EnvMode    string
	Output     string
	Error      string
}

var cfg Config

var rootCmd = &cobra.Command{
	Use:   "eidolon-server",
	Short: "Eidolon log server - receives and displays proxy command logs",
	Long: `Eidolon server listens for log messages from eidolon proxy instances
and displays them with colorized, formatted output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.NoColor {
			color.NoColor = true
		}

		server := NewServer(&cfg)
		return server.Run()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&cfg.Address, "address", "a", "0.0.0.0:9999", "Server listen address")
	rootCmd.Flags().BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")
	rootCmd.Flags().BoolVarP(&cfg.OnlyErrors, "only-errors", "e", false, "Show only non-zero exit codes")
	rootCmd.Flags().StringVarP(&cfg.Search, "search", "s", "", "Highlight matching text in output")
	rootCmd.Flags().StringVarP(&cfg.Filter, "filter", "f", "", "Skip commands containing this text in stdin/stdout/stderr")
	rootCmd.Flags().StringVarP(&cfg.EnvMode, "env-mode", "m", "diff", "Environment mode: all|diff|none")
	rootCmd.Flags().StringVar(&cfg.Output, "output", "/tmp/eidolon.txt", "Output log file (all commands)")
	rootCmd.Flags().StringVar(&cfg.Error, "error", "/tmp/eidolon_error.txt", "Error log file (non-zero exit only)")
}
