package Metrics

import (
	"context"
	"fmt"
	"log"
	"time"

	"agent/Middleware"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// 初始化 Kubernetes 客户端和 Metrics 客户端
func InitializeClients(kubeconfig string) (*kubernetes.Clientset, *metricsclient.Clientset, error) {
	//start := time.Now() // 开始计时

	// 构建 Kubernetes 配置
	var config *rest.Config
	var err error

	// 如果没有提供 kubeconfig 参数，则使用集群内的默认配置（InClusterConfig）
	if kubeconfig == "" {
		config, err = rest.InClusterConfig() // 从集群内部环境读取配置
		if err != nil {
			return nil, nil, fmt.Errorf("无法获取集群内部配置: %v", err)
		}
	} else {
		// 使用 kubeconfig 文件的配置
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, nil, fmt.Errorf("无法构建 Kubernetes 配置: %v", err)
		}
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("无法创建 Kubernetes 客户端: %v", err)
	}

	// 创建 Metrics 客户端
	metricsClient, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("无法创建 Metrics 客户端: %v", err)
	}

	//log.Printf("初始化 Kubernetes 和 Metrics 客户端耗时: %v", time.Since(start)) // 打印耗时
	return clientset, metricsClient, nil
}

// 获取所有 Pod 和容器的资源状态
func GetPodResources(clientset *kubernetes.Clientset, metricsClient *metricsclient.Clientset) ([]Middleware.ContainerResource, error) {
	//start := time.Now() // 记录获取 Pod 资源信息的开始时间

	var containerResources []Middleware.ContainerResource

	// 获取所有 namespaces 下的所有 Pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 Pods: %v", err)
	}

	// 获取所有 namespaces 下的所有 PodMetrics
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 PodMetrics: %v", err)
	}

	// 遍历所有 Pods 获取资源信息
	for _, pod := range pods.Items {
		controllerName := getControllerName(&pod)
		for _, container := range pod.Spec.Containers {
			// 获取 CPU 和内存限制与请求
			limitCpu := getCpuLimitOrRequest(container.Resources.Limits[corev1.ResourceCPU])
			limitMemory := getMemoryLimitOrRequest(container.Resources.Limits[corev1.ResourceMemory])
			requestCpu := getCpuLimitOrRequest(container.Resources.Requests[corev1.ResourceCPU])
			requestMemory := getMemoryLimitOrRequest(container.Resources.Requests[corev1.ResourceMemory])

			// 查找对应的 PodMetrics
			var cpuUsage, memUsage int64
			for _, podMetrics := range podMetrics.Items {
				if podMetrics.Name == pod.Name && podMetrics.Namespace == pod.Namespace {
					for _, containerMetrics := range podMetrics.Containers {
						if containerMetrics.Name == container.Name {
							// 获取 CPU 和内存使用情况
							cpuUsage = getCpuUsage(containerMetrics.Usage["cpu"])
							memUsage = getMemoryUsage(containerMetrics.Usage["memory"])
						}
					}
				}
			}

			// 获取容器的重启次数
			restartCount := getRestartCount(container.Name, pod.Status.ContainerStatuses)

			// 获取容器的上一次终止时间
			var lastTerminationTimeUnix int64
			var lastTerminationTime int64 // 用于存储时间差（以分钟为单位）
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == container.Name && containerStatus.LastTerminationState.Terminated != nil {
					// 转换为 Unix 毫秒时间戳
					lastTerminationTimeUnix = containerStatus.LastTerminationState.Terminated.FinishedAt.UnixMilli()

					// 计算当前时间与上次终止时间的时间差（单位：毫秒）
					now := time.Now().UnixMilli() // 当前时间的毫秒时间戳
					timeDiff := now - lastTerminationTimeUnix

					// 如果时间差大于等于一分钟，计算分钟数
					if timeDiff >= 60000 { // 如果大于等于一分钟
						lastTerminationTime = timeDiff / 60000 // 转换为分钟
					} else {
						lastTerminationTime = 0 // 小于一分钟，忽略
					}
				}
			}

			// 创建容器资源数据
			containerResource := Middleware.ContainerResource{
				Namespace:           pod.Namespace,
				PodName:             pod.Name,
				ControllerName:      controllerName,
				Container:           container.Name,
				LimitCpu:            limitCpu,
				LimitMemory:         limitMemory,
				RequestCpu:          requestCpu,
				RequestMemory:       requestMemory,
				UseCpu:              float64(cpuUsage) / 1000.0,
				UseMemory:           memUsage,
				RestartCount:        int(restartCount),   // 设置重启次数
				LastTerminationTime: lastTerminationTime, // 新增字段，转换为秒
			}

			// 添加到结果集中
			containerResources = append(containerResources, containerResource)
		}
	}

	//log.Printf("获取所有 Pod 资源信息总耗时: %v", time.Since(start)) // 打印获取 Pod 资源的总耗时
	return containerResources, nil
}

// 获取容器的重启时间戳
func getContainerRestartTime(containerName string, containerStatuses []corev1.ContainerStatus) *time.Time {
	for _, status := range containerStatuses {
		if status.Name == containerName {
			log.Printf("正在检查容器 %s 的状态", containerName)

			// 检查容器是否有终止状态
			if status.State.Terminated != nil {
				log.Printf("容器 %s 终止状态，原因: %s，结束时间: %v", containerName, status.State.Terminated.Reason, status.State.Terminated.FinishedAt)
				// 如果容器是由于错误或者终止而重启，返回重启时间
				return &status.State.Terminated.FinishedAt.Time
			}

			// 如果容器处于运行状态，输出日志
			if status.State.Running != nil {
				log.Printf("容器 %s 当前处于运行状态，启动时间: %v", containerName, status.State.Running.StartedAt)
			}

			// 如果容器处于等待状态，输出日志
			if status.State.Waiting != nil {
				log.Printf("容器 %s 当前处于等待状态，原因: %s", containerName, status.State.Waiting.Reason)
			}
		}
	}
	log.Printf("未找到容器 %s 的重启时间", containerName)
	return nil // 如果没有找到重启时间，则返回 nil
}

// 获取容器的重启次数
func getRestartCount(containerName string, containerStatuses []corev1.ContainerStatus) int32 {
	for _, status := range containerStatuses {
		if status.Name == containerName {
			return status.RestartCount // 返回容器的重启次数
		}
	}
	return 0 // 如果没有找到对应容器，返回 0
}

// 获取控制器名称（如 ReplicaSet、Deployment 等）
func getControllerName(pod *corev1.Pod) string {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" || ownerRef.Kind == "Deployment" || ownerRef.Kind == "StatefulSet" {
			return ownerRef.Name
		}
	}
	return "无控制器"
}

// 获取 CPU 的资源限制或请求（返回核心数）
func getCpuLimitOrRequest(quantity resource.Quantity) float64 {
	if quantity.IsZero() {
		return 0.0
	}
	return quantity.AsApproximateFloat64() // 返回核心数
}

// 获取内存的资源限制或请求（返回字节数）
func getMemoryLimitOrRequest(quantity resource.Quantity) int64 {
	if quantity.IsZero() {
		return 0
	}
	return quantity.Value() // 返回字节数
}

// 获取 CPU 使用量（返回毫核）
func getCpuUsage(cpu resource.Quantity) int64 {
	if cpu.IsZero() {
		return 0
	}
	return cpu.MilliValue() // 返回毫核
}

// 获取内存使用量（返回字节数）
func getMemoryUsage(memory resource.Quantity) int64 {
	if memory.IsZero() {
		return 0
	}
	return memory.Value() // 返回字节数
}
