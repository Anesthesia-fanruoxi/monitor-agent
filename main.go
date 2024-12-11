package main

import (
	"agent/Metrics"
	"agent/Middleware"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"time"
)

// 配置结构体
type Config struct {
	Agent struct {
		Project    string `yaml:"project"`
		MetricsURL string `yaml:"metrics_url"`
	} `yaml:"agent"`
	Metrics struct {
		Ssl    struct{ Enable bool } `yaml:"ssl"`
		Nginx  struct{ Enable bool } `yaml:"nginx"`
		Harbor struct{ Enable bool } `yaml:"harbor"`
		K8S    struct {
			Enable     bool   `yaml:"enable"`
			ConfigPath string `yaml:"config_path"`
		} `yaml:"k8s"`
	} `yaml:"metrics"`
	Encrypted string `yaml:"encrypted"` // 加密密钥
}

// 加载配置函数
func LoadConfig() (Config, error) {
	var config Config
	filePath := "config.yaml"

	// 检查配置文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Println("配置文件不存在，创建默认配置文件...")

		// 创建带注释的默认配置文件内容
		defaultConfig := `# 配置文件示例
# 基础配置
agent:

  # 项目名称，用于区分项目
  project: ""

  # 接受数据地址
  metrics_url: ""

# 是否开启采集,true为开启，false为不开启
metrics:

  # 是否开启采集ssl证书到期时间
  ssl:
    enable: false

  # 是否开启Nginx服务器信息采集
  nginx: 
    enable: false

  # 是否开启harbor服务信息采集
  harbor:
    enable: false

  # 是否开启采集k8s集群pod资源信息
  k8s: 
    enable: false
    # 开启之后需要填入路径，如果是当前路径直接写admin.conf,如果不是就写绝对路径
    config_path: ""

# 加密盐，数据加密传输
encrypted: ""`

		// 创建并写入配置文件
		file, err := os.Create(filePath)
		if err != nil {
			return config, err
		}
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {

			}
		}(file)

		// 写入带注释的默认配置内容
		if _, err := file.WriteString(defaultConfig); err != nil {
			return config, err
		}

		log.Println("默认配置文件已创建，请修改后重新运行程序。")
		return config, nil
	}

	// 如果配置文件存在，读取配置
	file, err := os.Open(filePath)
	if err != nil {
		return config, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}

	return config, nil
}

// 采集数据并发送数据的封装方法
func collectAndSendData(source string, data interface{}) {
	// 获取配置
	config, err := LoadConfig()
	if err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	// 使用配置中的信息，处理采集并发送数据
	metricsURL := config.Agent.MetricsURL
	project := config.Agent.Project
	key := []byte(config.Encrypted)

	// 异步发送数据
	go func() {
		// 发送数据
		if err := Middleware.SendData(metricsURL, project, data, key, source); err != nil {
			log.Printf("发送 %s 信息失败: %v", source, err)
		}
	}()
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	// 主循环，保持程序运行
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentSecond := time.Now().Second()

			// 检查是否在0秒或5秒时发送主机信息数据
			if currentSecond%5 == 0 {
				// 获取主机信息
				hostInfoSlice, err := Metrics.GetHostInfo()
				if err != nil {
					log.Printf("获取主机信息失败: %v", err)
					continue
				}
				// 调用封装方法发送主机信息数据
				collectAndSendData("hard", hostInfoSlice)

				// 获取心跳信息
				ActiveInfo, err := Metrics.IsActive(config.Agent.Project)
				if err != nil {
					log.Printf("获取心跳数据失败: %v", err)
					continue
				}
				// 调用封装方法发送心跳信息数据
				collectAndSendData("heart", ActiveInfo)

				// 如果 Nginx 采集被启用，发送 Nginx 信息
				if config.Metrics.Nginx.Enable {
					NginxInfo, err := Metrics.GetNginxInfo()
					if err != nil {
						log.Printf("获取 nginx 信息失败: %v", err)
						continue
					}
					// 调用封装方法发送 Nginx 信息数据
					collectAndSendData("nginx", NginxInfo)
				}

				// 如果 config.Metrics.K8S.Enable 为 true，发送 Kubernetes 信息
				if config.Metrics.K8S.Enable {
					clientset, metricsClient, err := Metrics.InitializeClients(config.Metrics.K8S.ConfigPath)
					if err != nil {
						log.Printf("初始化 Kubernetes 客户端失败: %v", err)
						continue
					}
					containerResources, err := Metrics.GetPodResources(clientset, metricsClient)
					if err != nil {
						log.Printf("获取 Kubernetes 资源信息失败: %v", err)
						continue
					}
					// 调用封装方法发送 Kubernetes 信息数据
					collectAndSendData("k8s", containerResources)

					controllerResources, err := Metrics.GetControllerResources(clientset)
					if err != nil {
						log.Printf("获取 Kubernetes 控制器资源信息失败: %v", err)
						continue
					}
					// 调用封装方法发送 Kubernetes 信息数据
					collectAndSendData("k8sController", controllerResources)
				}

				// 如果 config.Metrics.Ssl.Enable 为 true，发送 SSL 信息
				if config.Metrics.Ssl.Enable {
					SslInfos, err := Metrics.GetSslInfo()
					if err != nil {
						log.Printf("获取 Ssl 信息失败: %v", err)
						continue
					}
					var SslData []map[string]interface{}
					err = json.Unmarshal([]byte(SslInfos), &SslData)
					if err != nil {
						log.Printf("解析 Ssl 信息失败: %v", err)
						continue
					}
					// 调用封装方法发送 SSL 信息数据
					collectAndSendData("ssl", SslData)
				}
			}
		}
	}
}
