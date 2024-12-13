package Middleware

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex // 用于锁定并发执行

// 从服务器获取最新版本号
func getVersionFromServer(url string) (float64, error) {
	// 设置超时时间为5秒
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(url) // 发起 HTTP GET 请求
	if err != nil {
		return 0, fmt.Errorf("获取版本号失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 假设返回的版本号是纯文本（如 "1.1"）
	remoteVersionStr := string(body)

	// 将返回的版本号字符串转换为 float64
	remoteVersion, err := strconv.ParseFloat(strings.TrimSpace(remoteVersionStr), 64)
	if err != nil {
		return 0, fmt.Errorf("转换版本号失败: %v", err)
	}

	return remoteVersion, nil
}

// 判断本地版本和远程版本是否不匹配
func isVersionsNotMatching(localVersion, remoteVersion float64) bool {
	return localVersion != remoteVersion
}

// 创建更新脚本
func createUpdateScript(version float64, url string) error {
	// 脚本文件路径
	scriptPath := "./update.sh"
	// 拼接下载链接
	downloadURL := fmt.Sprintf("%s/agent/agent", url)

	// 脚本内容
	scriptContent := fmt.Sprintf(`# 更新脚本，用于版本 %.2f 的更新
echo "正在更新到版本 %.2f"
# 下载新版本
curl -o agent-%.2f -w "%%{http_code}" -s %s
if [ $? -ne 0 ]; then
    echo "下载新版本失败"
    exit 1
fi
# 删除当前版本
rm -f agent

# 替换新版本
mv agent-%.2f agent

# 加可执行权限
chmod +x agent

# 停止当前进程
cat work.pid |xargs kill 


echo "更新完成，当前版本为 %.2f"`, version, version, version, downloadURL, version, version)

	// 创建或覆盖脚本文件
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return fmt.Errorf("创建更新脚本失败: %v", err)
	}

	log.Printf("更新脚本已创建: %s", scriptPath)
	return nil
}

// 执行更新脚本
func executeUpdateScript() error {
	// 锁定，以避免并发冲突
	mu.Lock()
	defer mu.Unlock()

	// 脚本文件路径
	scriptPath := "./update.sh"

	// 执行脚本
	cmd := exec.Command("/bin/bash", scriptPath)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("执行更新脚本失败: %v", err)
	}

	log.Println("更新脚本执行成功")
	return nil
}

// 启动一个线程定期检查版本号
func CheckVersion(version string, url string) {
	// 检查脚本是否已存在，若存在则无需再次创建
	scriptPath := "./update.sh"
	if _, err := os.Stat(scriptPath); err == nil {
		err := os.Remove(scriptPath)
		if err != nil {
			log.Printf("删除文件失败: %v", err)
		}
	}
	for {
		remoteVersion, err := getVersionFromServer(fmt.Sprintf("%s/version", url))
		if err != nil {
			log.Printf("获取版本号失败: %v\n", err)
			time.Sleep(5 * time.Second) // 如果失败，稍后重试
			continue
		}
		localVersion, err := strconv.ParseFloat(version, 64)
		if err != nil {
			// 处理转换错误
			log.Println("转换错误:", err)
			return
		}

		// 比较本地版本与远程版本
		log.Printf("本地版本: %.2f, 远程版本: %.2f\n", localVersion, remoteVersion)
		if isVersionsNotMatching(localVersion, remoteVersion) {
			log.Printf("发现新版本! 本地版本: %.2f, 远程版本: %.2f\n", localVersion, remoteVersion)
			if err := createUpdateScript(remoteVersion, url); err != nil {
				log.Printf("创建更新脚本失败: %v", err)
				continue
			}

			if err := executeUpdateScript(); err != nil {
				log.Printf("执行更新脚本失败: %v", err)
				continue
			}
		}

		// 等待 10 秒钟再检查一次
		time.Sleep(10 * time.Second)
	}
}

func AutoChecks(Version string) {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	// 启动 goroutine
	go func() {
		for {
			CheckVersion(Version, config.Agent.MetricsURL)
			time.Sleep(5 * time.Second) // 每 5 秒检查一次
		}
	}()
}
