// Package main 测试 Kore gRPC 服务器和客户端
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yukin371/Kore/internal/client"
	"github.com/yukin371/Kore/internal/server"
)

func main() {
	log.SetFlags(log.Lshortfile)

	fmt.Println("=== Kore 2.0 gRPC 测试程序 ===")
	fmt.Println()

	// 1. 启动服务器
	fmt.Println("1. 启动服务器...")
	serverAddr, err := startTestServer()
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
	fmt.Printf("   ✓ 服务器已启动: %s\n\n", serverAddr)

	// 等待服务器就绪
	time.Sleep(500 * time.Millisecond)

	// 2. 连接服务器
	fmt.Println("2. 连接服务器...")
	koreClient, err := client.NewKoreClient(
		serverAddr,
		client.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatalf("连接服务器失败: %v", err)
	}
	defer koreClient.Close()
	fmt.Printf("   ✓ 已连接到服务器: %s\n\n", koreClient.ServerAddr())

	// 3. Ping 测试
	fmt.Println("3. Ping 测试...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := koreClient.Ping(ctx); err != nil {
		log.Printf("   ✗ Ping 失败: %v", err)
	} else {
		fmt.Println("   ✓ Ping 成功")
	}

	// 4. 创建会话测试（Phase 4 实现）
	fmt.Println("\n4. 创建会话测试...")
	fmt.Println("   (尚未实现 - Phase 4)")

	// 5. LSP 测试（Phase 3 实现）
	fmt.Println("\n5. LSP 测试...")
	fmt.Println("   (尚未实现 - Phase 3)")

	fmt.Println("\n=== 测试完成 ===")
}

// startTestServer 启动测试服务器
func startTestServer() (string, error) {
	// 自动检测端口
	addr, err := server.AutoDetectPort()
	if err != nil {
		return "", err
	}

	// 创建服务器
	koreServer := server.NewKoreServer(addr)

	// 启动服务器（在 goroutine 中）
	go func() {
		if err := koreServer.Start(); err != nil {
			log.Printf("服务器错误: %v", err)
		}
	}()

	return addr, nil
}
