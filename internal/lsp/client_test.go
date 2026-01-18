package lsp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yukin371/Kore/pkg/logger"
)

// TestClientInitialization 测试客户端初始化流程
func TestClientInitialization(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建客户端配置（使用 gopls 作为测试服务器）
	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	// 注意：这个测试需要系统安装 gopls
	// 在 CI 环境中可能跳过
	t.Run("Start and Initialize", func(t *testing.T) {
		if err := client.Start(ctx); err != nil {
			t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
			return
		}
		defer client.Close(ctx)

		if !client.initialized {
			t.Error("客户端未初始化")
		}

		// 检查能力
		if client.capabilities.CompletionProvider == nil {
			t.Log("服务器不支持补全")
		} else {
			t.Log("服务器支持补全")
		}

		if client.capabilities.DefinitionProvider == nil {
			t.Log("服务器不支持定义跳转")
		} else {
			t.Log("服务器支持定义跳转")
		}
	})
}

// TestDocumentSync 测试文档同步
func TestDocumentSync(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	if err := client.Start(ctx); err != nil {
		t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
		return
	}
	defer client.Close(ctx)

	uri := "file:///tmp/test.go"
	languageID := "go"
	text := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`

	t.Run("DidOpen", func(t *testing.T) {
		if err := client.DidOpen(ctx, uri, languageID, text); err != nil {
			t.Errorf("DidOpen 失败: %v", err)
		}

		doc, ok := client.GetDocument(uri)
		if !ok {
			t.Error("文档未打开")
		} else {
			if doc.Text != text {
				t.Error("文档内容不匹配")
			}
		}
	})

	t.Run("DidChange", func(t *testing.T) {
		changes := []TextDocumentContentChangeEvent{
			{
				Range: nil,
				Text:  text + "\n// Added comment",
			},
		}

		if err := client.DidChange(ctx, uri, changes); err != nil {
			t.Errorf("DidChange 失败: %v", err)
		}
	})

	t.Run("DidSave", func(t *testing.T) {
		if err := client.DidSave(ctx, uri, &text); err != nil {
			t.Errorf("DidSave 失败: %v", err)
		}
	})

	t.Run("DidClose", func(t *testing.T) {
		if err := client.DidClose(ctx, uri); err != nil {
			t.Errorf("DidClose 失败: %v", err)
		}

		_, ok := client.GetDocument(uri)
		if ok {
			t.Error("文档应该已关闭")
		}
	})
}

// TestCompletion 测试代码补全
func TestCompletion(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	if err := client.Start(ctx); err != nil {
		t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
		return
	}
	defer client.Close(ctx)

	if client.capabilities.CompletionProvider == nil {
		t.Skip("服务器不支持补全")
		return
	}

	// 打开测试文件
	uri := "file:///tmp/test_completion.go"
	text := `package main

import "fmt"

func main() {
	fmt.P
}
`
	client.DidOpen(ctx, uri, "go", text)

	// 请求补全
	completion, err := client.Completion(ctx, uri, Position{
		Line:      5,
		Character: 7,
	})

	if err != nil {
		t.Fatalf("补全请求失败: %v", err)
	}

	if completion == nil {
		t.Fatal("补全结果为空")
	}

	t.Logf("找到 %d 个补全项", len(completion.Items))

	// 验证应该有 Println 相关的补全
	foundPrintln := false
	for _, item := range completion.Items {
		if item.Label == "Println" {
			foundPrintln = true
			break
		}
	}

	if !foundPrintln {
		t.Log("警告：未找到 Println 补全项")
	}
}

// TestDefinition 测试定义跳转
func TestDefinition(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	if err := client.Start(ctx); err != nil {
		t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
		return
	}
	defer client.Close(ctx)

	if client.capabilities.DefinitionProvider == nil {
		t.Skip("服务器不支持定义跳转")
		return
	}

	// 打开测试文件
	uri := "file:///tmp/test_definition.go"
	text := `package main

func hello() {
	println("Hello")
}

func main() {
	hello()
}
`
	client.DidOpen(ctx, uri, "go", text)

	// 请求定义（在 main 函数的 hello() 调用处）
	locations, err := client.Definition(ctx, uri, Position{
		Line:      7,
		Character: 1,
	})

	if err != nil {
		t.Fatalf("定义请求失败: %v", err)
	}

	if len(locations) == 0 {
		t.Fatal("未找到定义")
	}

	t.Logf("找到 %d 个定义", len(locations))
	for i, loc := range locations {
		t.Logf("定义 %d: %s:%d:%d", i+1, loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestHover 测试悬停提示
func TestHover(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	if err := client.Start(ctx); err != nil {
		t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
		return
	}
	defer client.Close(ctx)

	if client.capabilities.HoverProvider == nil {
		t.Skip("服务器不支持悬停提示")
		return
	}

	// 打开测试文件
	uri := "file:///tmp/test_hover.go"
	text := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	client.DidOpen(ctx, uri, "go", text)

	// 请求悬停信息（在 fmt 上）
	hover, err := client.Hover(ctx, uri, Position{
		Line:      4,
		Character: 2,
	})

	if err != nil {
		t.Fatalf("悬停请求失败: %v", err)
	}

	if hover.Contents == nil {
		t.Log("无悬停内容")
		return
	}

	t.Logf("悬停内容: %+v", hover.Contents)
}

// TestReferences 测试查找引用
func TestReferences(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       "file:///E:/Github/kore-foundation",
	}

	client := NewClient(config, log)

	if err := client.Start(ctx); err != nil {
		t.Skipf("跳过测试：需要安装 gopls (错误: %v)", err)
		return
	}
	defer client.Close(ctx)

	if client.capabilities.ReferencesProvider == nil {
		t.Skip("服务器不支持查找引用")
		return
	}

	// 打开测试文件
	uri := "file:///tmp/test_references.go"
	text := `package main

func hello() {
	println("Hello")
}

func main() {
	hello()
	hello()
}
`
	client.DidOpen(ctx, uri, "go", text)

	// 请求引用（在 hello 函数定义处）
	locations, err := client.References(ctx, uri, Position{
		Line:      2,
		Character: 5,
	})

	if err != nil {
		t.Fatalf("引用请求失败: %v", err)
	}

	t.Logf("找到 %d 个引用", len(locations))

	// 应该找到 2 个引用（在 main 函数中）
	if len(locations) < 2 {
		t.Logf("警告：期望至少 2 个引用，找到 %d 个", len(locations))
	}

	for i, loc := range locations {
		t.Logf("引用 %d: %s:%d:%d", i+1, loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestUnmarshalLocations 测试位置解析
func TestUnmarshalLocations(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{
			name: "单个位置",
			input: map[string]interface{}{
				"uri": "file:///test.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 1, "character": 0},
					"end":   map[string]interface{}{"line": 1, "character": 5},
				},
			},
			expected: 1,
		},
		{
			name: "位置数组",
			input: []interface{}{
				map[string]interface{}{
					"uri": "file:///test1.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 1, "character": 0},
						"end":   map[string]interface{}{"line": 1, "character": 5},
					},
				},
				map[string]interface{}{
					"uri": "file:///test2.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 2, "character": 0},
						"end":   map[string]interface{}{"line": 2, "character": 5},
					},
				},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locations, err := unmarshalLocations(tt.input)
			if err != nil {
				t.Fatalf("unmarshalLocations 失败: %v", err)
			}

			if len(locations) != tt.expected {
				t.Errorf("期望 %d 个位置，得到 %d", tt.expected, len(locations))
			}
		})
	}
}

// TestUnmarshalParams 测试参数解析
func TestUnmarshalParams(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Value int   `json:"value"`
	}

	input := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	var result TestStruct
	err := unmarshalParams(input, &result)
	if err != nil {
		t.Fatalf("unmarshalParams 失败: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("期望 name='test'，得到 '%s'", result.Name)
	}

	if result.Value != 42 {
		t.Errorf("期望 value=42，得到 %d", result.Value)
	}
}

// TestMarshalJSON 测试 JSON 序列化
func TestMarshalJSON(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
		"number": 123,
	}

	result, err := marshalJSON(data)
	if err != nil {
		t.Fatalf("marshalJSON 失败: %v", err)
	}

	var check map[string]interface{}
	if err := json.Unmarshal(result, &check); err != nil {
		t.Fatalf("序列化结果无效: %v", err)
	}

	if check["key"] != "value" {
		t.Error("序列化数据不匹配")
	}
}

// TestUnmarshalJSON 测试 JSON 反序列化
func TestUnmarshalJSON(t *testing.T) {
	data := []byte(`{"name":"test","value":42}`)

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON 失败: %v", err)
	}

	if result.Name != "test" || result.Value != 42 {
		t.Error("反序列化数据不匹配")
	}
}
