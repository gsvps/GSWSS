package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gswss/gs-protocol/client/internal/app"
	"github.com/gswss/gs-protocol/client/internal/config"
	"github.com/gswss/gs-protocol/client/internal/log"
	"github.com/gswss/gs-protocol/client/internal/tray"
	"github.com/gswss/gs-protocol/client/internal/version"
)

var configPath string

func main() {
	if runtime.GOOS == "windows" && len(os.Args) == 1 {
		os.Args = append(os.Args, "tray")
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gs",
	Short: "GS Protocol client — secure transport proxy",
}

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Run GSWSS in the system tray (Windows)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tray.Run(configPath)
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the GS client proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if err := log.Init(cfg.LogLevel); err != nil {
			return err
		}
		defer log.Sync()

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		application := app.New(cfg)
		return application.Start(ctx)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show client running status",
	RunE: func(cmd *cobra.Command, args []string) error {
		running, pid, err := app.Status()
		if err != nil {
			return err
		}
		if running {
			fmt.Printf("GS client is running (PID %d)\n", pid)
		} else {
			fmt.Println("GS client is not running")
		}
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gs-client %s (build %s)\n", version.Version, version.Build)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	rootCmd.AddCommand(trayCmd, startCmd, statusCmd, versionCmd)
}
