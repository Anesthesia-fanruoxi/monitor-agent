package Metrics

import (
	"agent/Middleware"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1" // 需要导入 corev1 包
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// 初始化 Kubernetes 客户端和 Metrics 客户端
func InitializeClients(kubeconfig string) (*kubernetes.Clientset, *metricsclient.Clientset, error) {
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

	return clientset, metricsClient, nil
}

// 获取 Pod 资源信息
func GetPodResources(clientset *kubernetes.Clientset, metricsClient *metricsclient.Clientset) ([]Middleware.ContainerResource, error) {
	var containerResources []Middleware.ContainerResource

	// 获取所有 Pods
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 Pods: %v", err)
	}

	// 获取所有 PodMetrics
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有 PodMetrics: %v", err)
	}

	// 遍历 Pods 获取资源信息
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

			// 创建容器资源数据
			containerResource := Middleware.ContainerResource{
				Namespace:      pod.Namespace,
				PodName:        pod.Name,
				ControllerName: controllerName,
				Container:      container.Name,
				LimitCpu:       limitCpu,
				LimitMemory:    limitMemory,
				RequestCpu:     requestCpu,
				RequestMemory:  requestMemory,
				UseCpu:         float64(cpuUsage) / 1000.0, // 使用 float64 类型并转换为 CPU 核心数
				UseMemory:      memUsage,                   // 使用 int64 类型
			}

			// 将容器资源数据添加到切片
			containerResources = append(containerResources, containerResource)
		}
	}
	return containerResources, nil
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

// 获取所有命名空间下的所有控制器的副本信息
func GetAllNamespacesReplicasCount(clientset *kubernetes.Clientset) ([]Middleware.ControllerResource, error) {
	// 获取所有副本集 (ReplicaSets)
	replicaSets, err := clientset.AppsV1().ReplicaSets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有副本集: %v", err)
	}

	// 获取所有部署 (Deployments)
	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有部署: %v", err)
	}

	// 获取所有守护进程集 (DaemonSets)
	daemonSets, err := clientset.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("无法获取所有守护进程集: %v", err)
	}

	var controllerResources []Middleware.ControllerResource

	// 处理副本集 (ReplicaSets)
	for _, replicaSet := range replicaSets.Items {
		replicaCount := int32(0)
		if replicaSet.Spec.Replicas != nil {
			replicaCount = *replicaSet.Spec.Replicas
		}

		// 在 ControllerResource 中填充信息
		controllerResource := Middleware.ControllerResource{
			Namespace:      replicaSet.Namespace,
			ControllerName: "ReplicaSet",
			ReplicaCount:   replicaCount, // 添加副本集名称
		}
		controllerResources = append(controllerResources, controllerResource)
	}

	// 处理部署 (Deployments)
	for _, deployment := range deployments.Items {
		replicaCount := int32(0)
		if deployment.Spec.Replicas != nil {
			replicaCount = *deployment.Spec.Replicas
		}

		// 在 ControllerResource 中填充信息
		controllerResource := Middleware.ControllerResource{
			Namespace:      deployment.Namespace,
			ControllerName: "Deployment",
			ReplicaCount:   replicaCount, // 添加副本集名称
		}
		controllerResources = append(controllerResources, controllerResource)
	}

	// 处理守护进程集 (DaemonSets)
	for _, daemonSet := range daemonSets.Items {
		replicaCount := int32(0)
		// 使用 DaemonSet Status 中的 DesiredNumberScheduled 字段
		if daemonSet.Status.DesiredNumberScheduled > 0 {
			replicaCount = daemonSet.Status.DesiredNumberScheduled
		}

		// 在 ControllerResource 中填充信息
		controllerResource := Middleware.ControllerResource{
			Namespace:      daemonSet.Namespace,
			ControllerName: "DaemonSet",
			ReplicaCount:   replicaCount,
		}
		controllerResources = append(controllerResources, controllerResource)
	}

	// 返回结构化数据而非直接返回JSON字符串
	return controllerResources, nil
}
