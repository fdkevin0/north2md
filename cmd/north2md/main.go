package main

import (
	"log/slog"
	"os"

	"github.com/fdkevin0/north2md/internal/cli"
)

func main() {
	// Run CLI entrypoint.
	if err := cli.Execute(); err != nil {
		slog.Error("执行失败", "error", err)
		os.Exit(1)
	}
}
