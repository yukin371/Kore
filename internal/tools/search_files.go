package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SearchFilesTool 实现文件内容搜索功能
type SearchFilesTool struct {
	projectRoot string
	fs          *SecurityInterceptor
	timeout     time.Duration
	maxResults  int
	maxContext  int // 每个匹配结果的最大上下文行数
}

// NewSearchFilesTool 创建搜索工具
func NewSearchFilesTool(projectRoot string, fs *SecurityInterceptor) *SearchFilesTool {
	return &SearchFilesTool{
		projectRoot: projectRoot,
		fs:          fs,
		timeout:     30 * time.Second,
		maxResults:  100, // 最多返回 100 个结果
		maxContext:  2,   // 显示匹配行前后各 2 行
	}
}

// Name 返回工具名称
func (t *SearchFilesTool) Name() string {
	return "search_files"
}

// Description 返回工具描述
func (t *SearchFilesTool) Description() string {
	return "在项目文件中搜索文本内容。支持正则表达式，返回匹配的文件路径、行号和上下文。"
}

// Schema 返回工具的参数 JSON Schema
func (t *SearchFilesTool) Schema() string {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "要搜索的文本模式或正则表达式",
			},
			"file_pattern": map[string]interface{}{
				"type":        "string",
				"description": "文件名模式过滤（例如：*.go, *.md），可选",
			},
			"case_sensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "是否区分大小写，默认为 false",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "最大结果数量，默认为 100",
			},
		},
		"required": []string{"pattern"},
	}

	jsonBytes, _ := json.Marshal(schema)
	return string(jsonBytes)
}

// Execute 执行搜索
func (t *SearchFilesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 解析参数
	var params struct {
		Pattern        string `json:"pattern"`
		FilePattern    string `json:"file_pattern,omitempty"`
		CaseSensitive bool   `json:"case_sensitive,omitempty"`
		MaxResults     int    `json:"max_results,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	if params.Pattern == "" {
		return "", fmt.Errorf("pattern 参数不能为空")
	}

	// 设置默认值
	maxResults := t.maxResults
	if params.MaxResults > 0 && params.MaxResults < t.maxResults {
		maxResults = params.MaxResults
	}

	// 验证搜索模式（简单的安全检查）
	if err := t.validatePattern(params.Pattern); err != nil {
		return "", err
	}

	// 执行搜索
	results, err := t.search(ctx, params.Pattern, params.FilePattern, params.CaseSensitive, maxResults)
	if err != nil {
		return "", fmt.Errorf("搜索失败: %w", err)
	}

	// 格式化结果
	if len(results) == 0 {
		return "未找到匹配结果", nil
	}

	output := fmt.Sprintf("找到 %d 个匹配结果：\n\n", len(results))
	for _, result := range results {
		output += t.formatResult(result)
	}

	return output, nil
}

// validatePattern 验证搜索模式的安全性
func (t *SearchFilesTool) validatePattern(pattern string) error {
	// 检查是否包含危险的 regex 模式
	dangerousPatterns := []string{
		"(?<=.*",
		"(?=.*",
		"(?<!.*",
		"(?!.*",
		"(*PRINTE:",   // PCRE 危险模式
		"(*LIMIT:",    // PCRE 限制
	}

	for _, dangerous := range dangerousPatterns {
		if strings.Contains(pattern, dangerous) {
			return fmt.Errorf("搜索模式包含危险的表达式: %s", dangerous)
		}
	}

	// 尝试编译正则表达式以验证语法
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("无效的正则表达式: %w", err)
	}

	return nil
}

// SearchResult 表示一个搜索结果
type SearchResult struct {
	File     string   // 文件路径（相对路径）
	Line     int      // 匹配行号
	Content  string   // 匹配行的内容
	Context  []string // 上下文行
}

// search 执行搜索
func (t *SearchFilesTool) search(ctx context.Context, pattern, filePattern string, caseSensitive bool, maxResults int) ([]SearchResult, error) {
	// 首先尝试使用 ripgrep（如果可用）
	if results, err := t.searchWithRipgrep(ctx, pattern, filePattern, caseSensitive, maxResults); err == nil {
		return results, nil
	}

	// 回退到纯 Go 实现
	return t.searchWithGo(ctx, pattern, filePattern, caseSensitive, maxResults)
}

// searchWithRipgrep 使用 ripgrep 执行搜索
func (t *SearchFilesTool) searchWithRipgrep(ctx context.Context, pattern, filePattern string, caseSensitive bool, maxResults int) ([]SearchResult, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// 构建 ripgrep 命令
	args := t.buildRipgrepArgs(pattern, filePattern, caseSensitive, maxResults)

	// 根据操作系统选择命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "rg", args...)
	} else {
		cmd = exec.CommandContext(ctx, "rg", args...)
	}

	cmd.Dir = t.projectRoot

	// 执行命令
	output, err := cmd.Output()
	if err != nil {
		// ripgrep 不可用或其他错误
		return nil, fmt.Errorf("ripgrep 执行失败: %w", err)
	}

	// 解析 ripgrep 输出
	return t.parseRipgrepOutput(string(output)), nil
}

// buildRipgrepArgs 构建 ripgrep 命令参数
func (t *SearchFilesTool) buildRipgrepArgs(pattern, filePattern string, caseSensitive bool, maxResults int) []string {
	args := []string{
		"--json",           // JSON 格式输出
		"--line-number",    // 显示行号
		"--with-filename",  // 显示文件名
		"--context", "2",   // 显示上下文
		"--max-count", fmt.Sprintf("%d", maxResults/t.maxContext), // 限制每个文件的匹配数
	}

	if !caseSensitive {
		args = append(args, "--ignore-case")
	}

	if filePattern != "" {
		args = append(args, "--glob", filePattern)
	}

	// 添加搜索模式
	args = append(args, pattern)

	// 添加搜索路径（当前目录）
	args = append(args, ".")

	return args
}

// parseRipgrepOutput 解析 ripgrep JSON 输出
func (t *SearchFilesTool) parseRipgrepOutput(output string) []SearchResult {
	lines := strings.Split(output, "\n")
	results := make([]SearchResult, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// ripgrep JSON 格式解析
		var rgResult struct {
			Type     struct {
				Type string `json:"type"`
			} `json:"type"`
			Data struct {
				Path   struct {
					Text string `json:"path"`
				} `json:"path"`
				Lines struct {
					Text string `json:"lines"`
				} `json:"lines"`
				LineNumber int `json:"line_number"`
			} `json:"data"`
		}

		if err := json.Unmarshal([]byte(line), &rgResult); err != nil {
			continue
		}

		if rgResult.Type.Type == "match" {
			results = append(results, SearchResult{
				File:    rgResult.Data.Path.Text,
				Line:    rgResult.Data.LineNumber,
				Content: strings.TrimSpace(rgResult.Data.Lines.Text),
			})
		}
	}

	return results
}

// searchWithGo 使用纯 Go 实现搜索（回退方案）
func (t *SearchFilesTool) searchWithGo(ctx context.Context, pattern, filePattern string, caseSensitive bool, maxResults int) ([]SearchResult, error) {
	// 编译正则表达式
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("正则表达式编译失败: %w", err)
	}

	results := make([]SearchResult, 0, maxResults)

	// 遍历项目文件
	err = filepath.Walk(t.projectRoot, func(path string, info os.FileInfo, err error) error {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil // 跳过无法访问的文件
		}

		if info.IsDir() {
			return nil
		}

		// 检查文件模式
		if filePattern != "" {
			matched, err := filepath.Match(filePattern, filepath.Base(path))
			if err != nil || !matched {
				return nil
			}
		}

		// 读取文件内容并搜索
		fileResults, err := t.searchFile(path, re, maxResults-len(results))
		if err != nil {
			return nil // 跳过无法读取的文件
		}

		results = append(results, fileResults...)

		// 检查是否已达到最大结果数
		if len(results) >= maxResults {
			return fmt.Errorf("达到最大结果数")
		}

		return nil
	})

	if err != nil && err.Error() != "达到最大结果数" {
		return nil, err
	}

	return results, nil
}

// searchFile 搜索单个文件
func (t *SearchFilesTool) searchFile(filePath string, re *regexp.Regexp, maxResults int) ([]SearchResult, error) {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	results := make([]SearchResult, 0, maxResults)
	relPath, _ := filepath.Rel(t.projectRoot, filePath)

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			results = append(results, SearchResult{
				File:    relPath,
				Line:    lineNum,
				Content: strings.TrimSpace(line),
			})

			if len(results) >= maxResults {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// formatResult 格式化搜索结果
func (t *SearchFilesTool) formatResult(result SearchResult) string {
	return fmt.Sprintf("📄 %s:%d\n%s\n\n",
		result.File,
		result.Line,
		result.Content,
	)
}
