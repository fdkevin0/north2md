package main

import (
	"fmt"
	"os"
)

func main() {
	// 执行命令行程序
	if err := Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
