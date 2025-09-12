package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SystemStats struct {
	Timestamp string        `json:"timestamp"`
	CPU       CPUStats      `json:"cpu"`
	Memory    MemoryStats   `json:"memory"`
	Storage   StorageStats  `json:"storage"`
	Network   *NetworkStats `json:"network,omitempty"`
	Docker    *DockerStats  `json:"docker,omitempty"`
}

type CPUStats struct {
	Cores        int     `json:"cores"`
	Load1Min     float64 `json:"load_1min"`
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStats struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

type StorageStats struct {
	TotalGB      float64 `json:"total_gb"`
	UsedGB       float64 `json:"used_gb"`
	AvailableGB  float64 `json:"available_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

type NetworkStats struct {
	Interface  string  `json:"interface"`
	RxKBPerSec float64 `json:"rx_kb_per_sec"`
	TxKBPerSec float64 `json:"tx_kb_per_sec"`
}

type DockerStats struct {
	ContainersRunning int `json:"containers_running"`
	ContainersTotal   int `json:"containers_total"`
}

func (r *remote) getSystemStats(jsonOutput bool) error {
	return withSSHClient(r.address, func(client *ssh.Client) error {
		fmt.Println("gathering system statistics")

		stats := SystemStats{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		}

		// get cpu stats
		err := getCPUStats(client, &stats.CPU)
		if err != nil {
			return fmt.Errorf("failed to get cpu stats: %v", err)
		}

		// get memory stats
		err = getMemoryStats(client, &stats.Memory)
		if err != nil {
			return fmt.Errorf("failed to get memory stats: %v", err)
		}

		// get storage stats
		err = getStorageStats(client, &stats.Storage)
		if err != nil {
			return fmt.Errorf("failed to get storage stats: %v", err)
		}

		// get network stats (optional)
		networkStats, err := getNetworkStats(client)
		if err == nil {
			stats.Network = networkStats
		}

		// get docker stats (optional)
		dockerStats, err := getDockerStats(client)
		if err == nil {
			stats.Docker = dockerStats
		}

		if jsonOutput {
			jsonBytes, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal json: %v", err)
			}
			fmt.Println(string(jsonBytes))
		} else {
			printHumanReadableStats(&stats)
		}

		return nil
	})
}

func getCPUStats(client *ssh.Client, cpu *CPUStats) error {
	// get cpu cores
	coresOut, _, err := runSSHCommand(client, "nproc")
	if err != nil {
		return err
	}
	cpu.Cores, err = strconv.Atoi(strings.TrimSpace(coresOut))
	if err != nil {
		return err
	}

	// get load average
	loadOut, _, err := runSSHCommand(client, "uptime")
	if err != nil {
		return err
	}

	// parse load average from uptime output
	parts := strings.Split(loadOut, "load average:")
	if len(parts) < 2 {
		return fmt.Errorf("failed to parse load average")
	}

	loadParts := strings.Split(strings.TrimSpace(parts[1]), ",")
	if len(loadParts) < 1 {
		return fmt.Errorf("failed to parse load average")
	}

	cpu.Load1Min, err = strconv.ParseFloat(strings.TrimSpace(loadParts[0]), 64)
	if err != nil {
		return err
	}

	cpu.UsagePercent = (cpu.Load1Min / float64(cpu.Cores)) * 100.0

	return nil
}

func getMemoryStats(client *ssh.Client, mem *MemoryStats) error {
	memOut, _, err := runSSHCommand(client, "free -b")
	if err != nil {
		return err
	}

	lines := strings.Split(memOut, "\n")
	var memLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			memLine = line
			break
		}
	}

	if memLine == "" {
		return fmt.Errorf("failed to find memory line in free output")
	}

	fields := strings.Fields(memLine)
	if len(fields) < 7 {
		return fmt.Errorf("unexpected format in free output")
	}

	total, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return err
	}

	used, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return err
	}

	available, err := strconv.ParseInt(fields[6], 10, 64)
	if err != nil {
		return err
	}

	mem.TotalGB = float64(total) / (1024 * 1024 * 1024)
	mem.UsedGB = float64(used) / (1024 * 1024 * 1024)
	mem.AvailableGB = float64(available) / (1024 * 1024 * 1024)
	mem.UsagePercent = (float64(used) / float64(total)) * 100.0

	return nil
}

func getStorageStats(client *ssh.Client, storage *StorageStats) error {
	diskOut, _, err := runSSHCommand(client, "df -B1 /")
	if err != nil {
		return err
	}

	lines := strings.Split(diskOut, "\n")
	if len(lines) < 2 {
		return fmt.Errorf("unexpected format in df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return fmt.Errorf("unexpected format in df output")
	}

	total, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return err
	}

	used, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return err
	}

	available, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return err
	}

	usageStr := strings.TrimSuffix(fields[4], "%")
	usagePercent, err := strconv.ParseFloat(usageStr, 64)
	if err != nil {
		return err
	}

	storage.TotalGB = float64(total) / (1024 * 1024 * 1024)
	storage.UsedGB = float64(used) / (1024 * 1024 * 1024)
	storage.AvailableGB = float64(available) / (1024 * 1024 * 1024)
	storage.UsagePercent = usagePercent

	return nil
}

func getNetworkStats(client *ssh.Client) (*NetworkStats, error) {
	// get default network interface from /proc/net/route
	routeOut, _, err := runSSHCommand(client, "awk 'NR>1 && $2==\"00000000\" {print $1; exit}' /proc/net/route")
	if err != nil {
		return nil, fmt.Errorf("failed to get network interface: %v", err)
	}

	iface := strings.TrimSpace(routeOut)
	if iface == "" {
		return nil, fmt.Errorf("no default interface found")
	}

	// get network stats with two samples
	stats1Out, _, err := runSSHCommand(client, fmt.Sprintf("cat /proc/net/dev | grep %s | awk '{print $2 \" \" $10}'", iface))
	if err != nil {
		return nil, err
	}

	// wait 1 second for second sample
	time.Sleep(1 * time.Second)

	stats2Out, _, err := runSSHCommand(client, fmt.Sprintf("cat /proc/net/dev | grep %s | awk '{print $2 \" \" $10}'", iface))
	if err != nil {
		return nil, err
	}

	// parse first sample
	fields1 := strings.Fields(strings.TrimSpace(stats1Out))
	if len(fields1) < 2 {
		return nil, fmt.Errorf("failed to parse network stats")
	}

	rx1, err := strconv.ParseInt(fields1[0], 10, 64)
	if err != nil {
		return nil, err
	}

	tx1, err := strconv.ParseInt(fields1[1], 10, 64)
	if err != nil {
		return nil, err
	}

	// parse second sample
	fields2 := strings.Fields(strings.TrimSpace(stats2Out))
	if len(fields2) < 2 {
		return nil, fmt.Errorf("failed to parse network stats")
	}

	rx2, err := strconv.ParseInt(fields2[0], 10, 64)
	if err != nil {
		return nil, err
	}

	tx2, err := strconv.ParseInt(fields2[1], 10, 64)
	if err != nil {
		return nil, err
	}

	// calculate kb/s
	rxKBPerSec := float64(rx2-rx1) / 1024.0
	txKBPerSec := float64(tx2-tx1) / 1024.0

	return &NetworkStats{
		Interface:  iface,
		RxKBPerSec: rxKBPerSec,
		TxKBPerSec: txKBPerSec,
	}, nil
}

func getDockerStats(client *ssh.Client) (*DockerStats, error) {
	// check if docker is available
	_, _, err := runSSHCommand(client, "docker --version")
	if err != nil {
		return nil, err
	}

	// get running containers
	runningOut, _, err := runSSHCommand(client, "docker ps -q | wc -l")
	if err != nil {
		return nil, err
	}

	running, err := strconv.Atoi(strings.TrimSpace(runningOut))
	if err != nil {
		return nil, err
	}

	// get total containers
	totalOut, _, err := runSSHCommand(client, "docker ps -aq | wc -l")
	if err != nil {
		return nil, err
	}

	total, err := strconv.Atoi(strings.TrimSpace(totalOut))
	if err != nil {
		return nil, err
	}

	return &DockerStats{
		ContainersRunning: running,
		ContainersTotal:   total,
	}, nil
}

func printHumanReadableStats(stats *SystemStats) {
	fmt.Printf("=== System Stats - %s ===\n", stats.Timestamp)
	fmt.Println()

	fmt.Println("CPU:")
	fmt.Printf("  Cores: %d\n", stats.CPU.Cores)
	fmt.Printf("  Load (1min): %.2f\n", stats.CPU.Load1Min)
	fmt.Printf("  Usage: %.1f%%\n", stats.CPU.UsagePercent)
	fmt.Println()

	fmt.Println("Memory:")
	fmt.Printf("  Total: %.1fGB\n", stats.Memory.TotalGB)
	fmt.Printf("  Used: %.1fGB (%.1f%%)\n", stats.Memory.UsedGB, stats.Memory.UsagePercent)
	fmt.Printf("  Available: %.1fGB\n", stats.Memory.AvailableGB)
	fmt.Println()

	fmt.Println("Storage (/):")
	fmt.Printf("  Total: %.1fGB\n", stats.Storage.TotalGB)
	fmt.Printf("  Used: %.1fGB (%.1f%%)\n", stats.Storage.UsedGB, stats.Storage.UsagePercent)
	fmt.Printf("  Available: %.1fGB\n", stats.Storage.AvailableGB)
	fmt.Println()

	if stats.Network != nil {
		fmt.Printf("Network (%s):\n", stats.Network.Interface)
		fmt.Printf("  RX: %.1f KB/s\n", stats.Network.RxKBPerSec)
		fmt.Printf("  TX: %.1f KB/s\n", stats.Network.TxKBPerSec)
		fmt.Println()
	}

	if stats.Docker != nil {
		fmt.Println("Docker:")
		fmt.Printf("  Running containers: %d\n", stats.Docker.ContainersRunning)
		fmt.Printf("  Total containers: %d\n", stats.Docker.ContainersTotal)
	}
}
