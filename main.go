package main

import (
	"log/slog"
	"os"
)

func main() {
	// 执行命令行程序
	if err := Execute(); err != nil {
		slog.Error("执行失败", "error", err)
		os.Exit(1)
	}
}
