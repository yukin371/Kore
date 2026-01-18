// Kore 2.0 Server - gRPC 服务器主程序
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/yukin371/Kore/internal/server"
)

var (
	version = "dev"
	commit  = "unknown"
)

var (
	listenAddr = flag.String("listen", "auto", "Server listen address (auto, 127.0.0.1:8080, or unix socket path)")
	showVersion = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()

	// 显示版本
	if *showVersion {
		fmt.Printf("Kore Server v%s (commit %s)\n", version, commit)
		return
	}

	log.Printf("Starting Kore Server v%s (commit %s)", version, commit)

	// 自动检测地址
	addr := *listenAddr
	if addr == "auto" {
		// 尝试 Unix Socket（Linux/macOS）
		if socketPath, err := server.CreateTempUnixSocket(); err == nil {
			addr = socketPath
			log.Printf("Using Unix socket: %s", addr)
		} else {
			// 降级到 TCP
			if tcpAddr, err := server.AutoDetectPort(); err == nil {
				addr = tcpAddr
				log.Printf("Using TCP: %s", addr)
			} else {
				log.Fatalf("Failed to auto-detect address: %v", err)
			}
		}
	}

	// 创建服务器
	koreServer := server.NewKoreServer(addr)

	// 启动服务器
	if err := koreServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Server started on %s", koreServer.Addr())

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")

	// 优雅关闭
	if err := koreServer.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	log.Println("Server stopped gracefully")
}

// resolveAutoAddr 自动解析地址
func resolveAutoAddr() (string, error) {
	// 1. 尝试 Unix Socket（仅 Linux/macOS）
	if path, err := detectUnixSocket(); err == nil {
		return path, nil
	}

	// 2. 降级到 TCP localhost
	return detectTCPAddr()
}

// detectUnixSocket 检测 Unix Socket 支持
func detectUnixSocket() (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("unix socket not supported on windows")
	}

	dir := os.TempDir()
	path := fmt.Sprintf("%s/kore-server-%d.sock", dir, os.Getpid())

	// 清理旧文件
	os.Remove(path)

	return path, nil
}

// detectTCPAddr 自动检测可用 TCP 端口
func detectTCPAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return fmt.Sprintf("127.0.0.1:%d", addr.Port), nil
}
