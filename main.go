package main

import (
	"agent/Collect"
	"agent/Middleware"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

var Version string // 版本号变量

const (
	DAEMON  = "daemon"
	FOREVER = "forever"
	PIDFILE = "work.pid"
)

func main() {
	if Version == "" {
		Version = "1.0" // 默认版本号
	}
	log.Printf("当前版本号：%s\n", Version)
	daemon := flag.Bool(DAEMON, false, "run in work")
	forever := flag.Bool(FOREVER, false, "run forever")
	flag.Parse()

	if *daemon {
		subProcess(stripSlice(os.Args, "-"+DAEMON))
		os.Exit(0)
	} else if *forever {
		for {
			cmd := subProcess(stripSlice(os.Args, "-"+FOREVER))
			cmd.Wait()
		}
		os.Exit(0)
	} else {
		writePID() // 在主程序中记录 PID
		work()
	}
}

func writePID() {
	pid := os.Getpid() // 获取当前进程 PID
	file, err := os.Create(PIDFILE)
	if err != nil {
		log.Fatalf("无法创建 PID 文件: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	_, err = file.WriteString(fmt.Sprintf("%d\n", pid))
	if err != nil {
		log.Fatalf("无法写入 PID 文件: %v", err)
	}
}

func subProcess(args []string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	return cmd
}

func stripSlice(slice []string, element string) []string {
	for i := 0; i < len(slice); {
		if slice[i] == element {
			slice = append(slice[:i], slice[i+1:]...)
		} else {
			i++
		}
	}
	return slice
}

func work() {
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
		log.Printf("已开启自动更新")
		go Middleware.AutoChecks(Version)
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
