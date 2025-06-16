package rs485

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunIEC101Agent 启动 IEC-101 Agent，并拦截 SIGINT/SIGTERM，在收到信号时优雅关闭。
// 它会阻塞直到收到退出信号。
func RunIEC101Agent(cfgPath string) {
	// 父 Context：监听 SIGINT/SIGTERM
	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	// 用于真正控制 Agent 的 Context，可以被 signal 或内部其他逻辑 cancel
	ctxAgent, cancelAgent := context.WithCancel(sigCtx)
	defer cancelAgent()

	// 1. 检查配置文件是否存在
	if _, err := os.Stat(cfgPath); err != nil {
		log.Fatalf("无法访问配置文件 '%s': %v", cfgPath, err)
	}

	// 2. 启动 IEC-101 Agent 协程
	go func() {
		if err := StartIEC101Agent(ctxAgent, cfgPath); err != nil {
			log.Fatalf("IEC-101 Agent 启动失败: %v", err)
		}
	}()

	log.Println("IEC-101 Agent 已启动")

	// 3. 等待 SIGINT/SIGTERM
	<-sigCtx.Done()
	log.Println("收到终止信号，正在关闭 IEC-101 Agent...")

	// 4. 取消 Agent 的 Context，让 StartIEC101Agent 返回
	cancelAgent()

	// 5. 等待一点时间让 Agent 关闭
	time.Sleep(1 * time.Second)
	log.Println("IEC-101 Agent 已退出")
}

//     RunIEC101Agent("./Emd.Agent.IEC101.E0.conf")
