package Collect

import (
	"agent/Metrics"
	"agent/Middleware"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

// 采集数据并发送数据的封装方法
func CollectAndSendMetrics(Version string, config Middleware.ConfigFile) {
	currentSecond := time.Now().Second()

	// 检查是否在0秒或5秒时发送主机信息数据
	if currentSecond%5 == 0 {
		// 获取主机信息
		hostInfoSlice, err := Metrics.GetHostInfo()
		if err != nil {
			log.Printf("获取主机信息失败: %v", err)
			return
		}
		// 调用封装方法发送主机信息数据
		CollectAndSendData("hard", hostInfoSlice)

		// 转换版本号为浮动类型
		versionFloat, err := strconv.ParseFloat(Version, 64)
		if err != nil {
			log.Printf("转换版本号失败: %v", err)
			return
		}

		// 获取心跳信息
		ActiveInfo, err := Metrics.IsActive(config.Agent.Project, versionFloat)
		if err != nil {
			log.Printf("获取心跳数据失败: %v", err)
			return
		}
		// 调用封装方法发送心跳信息数据
		CollectAndSendData("heart", ActiveInfo)

		// 如果 Nginx 采集被启用，发送 Nginx 信息
		if config.Metrics.Nginx.Enable {
			NginxInfo, err := Metrics.GetNginxInfo()
			if err != nil {
				log.Printf("获取 nginx 信息失败: %v", err)
				return
			}
			// 调用封装方法发送 Nginx 信息数据
			CollectAndSendData("nginx", NginxInfo)
		}

		// 如果 config.Metrics.K8S.Enable 为 true，发送 Kubernetes 信息
		if config.Metrics.K8S.Enable {
			clientset, metricsClient, err := Metrics.InitializeClients(config.Metrics.K8S.ConfigPath)
			if err != nil {
				log.Printf("初始化 Kubernetes 客户端失败: %v", err)
				return
			}
			containerResources, err := Metrics.GetPodResources(clientset, metricsClient)
			if err != nil {
				log.Printf("获取 Kubernetes 资源信息失败: %v", err)
				return
			}
			// 调用封装方法发送 Kubernetes 信息数据
			CollectAndSendData("k8s", containerResources)

			controllerResources, err := Metrics.GetControllerResources(clientset)
			if err != nil {
				log.Printf("获取 Kubernetes 控制器资源信息失败: %v", err)
				return
			}
			// 调用封装方法发送 Kubernetes 信息数据
			CollectAndSendData("k8sController", controllerResources)
		}

		// 如果 config.Metrics.Ssl.Enable 为 true，发送 SSL 信息
		if config.Metrics.Ssl.Enable {
			SslInfos, err := Metrics.GetSslInfo()
			if err != nil {
				log.Printf("获取 Ssl 信息失败: %v", err)
				return
			}
			var SslData []map[string]interface{}
			err = json.Unmarshal([]byte(SslInfos), &SslData)
			if err != nil {
				log.Printf("解析 Ssl 信息失败: %v", err)
				return
			}
			// 调用封装方法发送 SSL 信息数据
			CollectAndSendData("ssl", SslData)
		}
	}
}

// 采集数据并发送数据的封装方法
func CollectAndSendData(source string, data interface{}) {
	// 获取配置
	config, err := Middleware.LoadConfig()
	if err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	// 使用配置中的信息，处理采集并发送数据
	metricsURL := fmt.Sprintf("%s/metrics_data", config.Agent.MetricsURL)
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
