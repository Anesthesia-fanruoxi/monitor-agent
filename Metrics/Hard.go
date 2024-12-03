package Metrics

import (
	"agent/Middleware"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ============================================获取基本硬件信息
// 获取 CPU 使用率（通过读取 /proc/stat）
func getCPUPercent() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 {
		fields := strings.Fields(lines[0])
		if len(fields) >= 5 {
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			nice, _ := strconv.ParseUint(fields[2], 10, 64)
			system, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)

			total := user + nice + system + idle
			return 100 * float64(total-idle) / float64(total), nil
		}
	}
	return 0, fmt.Errorf("无法读取 CPU 信息")
}

// 获取磁盘信息（通过 df 命令）
func getDiskInfo() (uint64, uint64, uint64, float64, error) {
	cmd := exec.Command("df", "-B1", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 1 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 4 {
			total, _ := strconv.ParseUint(fields[1], 10, 64)
			used, _ := strconv.ParseUint(fields[2], 10, 64)
			free, _ := strconv.ParseUint(fields[3], 10, 64)
			usedPercent := (float64(used) / float64(total)) * 100
			return total, used, free, usedPercent, nil
		}
	}
	return 0, 0, 0, 0, fmt.Errorf("无法读取磁盘信息")
}

// 获取内存信息
func getMemoryInfo() (uint64, uint64, uint64, uint64, uint64, uint64, uint64, float64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	var total, free, buffered, cached, shared, available uint64

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			value, _ := strconv.ParseUint(fields[1], 10, 64)
			switch fields[0] {
			case "MemTotal:":
				total = value
			case "MemFree:":
				free = value
			case "Buffers:":
				buffered = value
			case "Cached:":
				cached = value
			case "Shmem:":
				shared = value
			case "MemAvailable:":
				available = value
			}
		}
	}

	// 计算已用内存
	used := total - free - buffered - cached
	usedPercent := (float64(used) / float64(total)) * 100

	return total, used, free, buffered, cached, shared, available, usedPercent, nil
}

// 获取负载信息
func getLoadInfo() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		load1, _ := strconv.ParseFloat(fields[0], 64)
		load5, _ := strconv.ParseFloat(fields[1], 64)
		load15, _ := strconv.ParseFloat(fields[2], 64)
		return load1, load5, load15, nil
	}
	return 0, 0, 0, fmt.Errorf("无法读取负载信息")
}

func GetHostName() (string, error) {
	data, err := os.ReadFile("/etc/hostname")
	if err != nil {
		return "", err
	}
	// 去除换行符和空格
	return strings.TrimSpace(string(data)), nil
}

// 获取完整的系统信息
func GetHostInfo() ([]Middleware.FlatSystemInfo, error) {
	cpuPercent, err := getCPUPercent()
	if err != nil {
		return nil, err
	}

	diskTotal, diskUsed, diskFree, diskUsedPercent, err := getDiskInfo()
	if err != nil {
		return nil, err
	}

	// 获取内存信息，包含缓存和共享内存
	memoryTotal, memoryUsed, memoryFree, memoryBuffered, memoryCached, memoryShared, memoryAvailable, memoryUsedPercent, err := getMemoryInfo()
	if err != nil {
		return nil, err
	}

	cpu_load1, cpu_load5, cpu_load15, err := getLoadInfo()
	if err != nil {
		return nil, err
	}

	hostName, err := GetHostName()
	if err != nil {
		return nil, err
	}

	// 将主机信息放入切片中
	hostInfo := Middleware.FlatSystemInfo{
		CPUPercent:        cpuPercent,
		DiskTotal:         diskTotal,
		DiskUsed:          diskUsed,
		DiskFree:          diskFree,
		DiskUsedPercent:   diskUsedPercent,
		MemoryTotal:       memoryTotal,
		MemoryUsed:        memoryUsed,
		MemoryFree:        memoryFree,
		MemoryBuffered:    memoryBuffered,
		MemoryCached:      memoryCached,
		MemoryShared:      memoryShared,
		MemoryAvailable:   memoryAvailable,
		MemoryUsedPercent: memoryUsedPercent,
		CPULoad1:          cpu_load1,
		CPULoad5:          cpu_load5,
		CPULoad15:         cpu_load15,
		HostName:          hostName,
	}
	// 返回切片
	return []Middleware.FlatSystemInfo{hostInfo}, nil

}
