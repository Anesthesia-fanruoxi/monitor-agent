package main

import (
	"agent/Collect"
	"agent/Middleware"
	"log"
	"time"
)

var Version string // 版本号变量

func main() {
	if Version == "" {
		Version = "0.0.1" // 默认版本号
	}
	log.Printf("当前版本号：%s\n", Version)

	// 加载配置
	config, err := Middleware.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 主循环，保持程序运行
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 如果启用了自动更新，启动心跳检查
	if config.Agent.AutoUpdate {
		go Middleware.StartHeartbeatChecks(Version)
	}

	// 主循环处理数据收集和发送
	for {
		select {
		case <-ticker.C:
			// 收集并发送数据
			Collect.CollectAndSendMetrics(Version, config)
		}
	}
}
