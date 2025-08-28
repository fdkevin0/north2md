package main

import (
	"strings"
)

// Common utility functions shared across the codebase

// Pre-compiled character replacement maps for performance
var markdownReplacements = map[string]string{
	"\\": "\\\\",
	"`":  "\\`",
	"*":  "\\*",
	"_":  "\\_",
	"{":  "\\{",
	"}":  "\\}",
	"[":  "\\[",
	"]":  "\\]",
	"(":  "\\(",
	")":  "\\)",
	"#":  "\\#",
	"+":  "\\+",
	"-":  "\\-",
	".":  "\\.",
	"!":  "\\!",
	"|":  "\\|",
}

// EscapeMarkdown 高效转义Markdown特殊字符
func EscapeMarkdown(text string) string {
	// 使用strings.Builder模拟strings.ReplaceAll的性能，但更高效
	var result strings.Builder
	result.Grow(len(text) * 2) // 预分配内存，避免多次分配

	for _, char := range text {
		str := string(char)
		if replacement, exists := markdownReplacements[str]; exists {
			result.WriteString(replacement)
		} else {
			result.WriteString(str)
		}
	}

	return result.String()
}

// TruncateText 截断文本到指定长度并添加省略号
func TruncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// CleanHTMLContent 高效清理HTML内容
func CleanHTMLContent(str string) string {
	// 单次操作清理前后空白和换行
	return strings.Trim(str, " \n\r\t")
}

// ReplaceNewlines 替换换行符为指定字符
func ReplaceNewlines(text, replacement string) string {
	return strings.ReplaceAll(text, "\n", replacement)
}
