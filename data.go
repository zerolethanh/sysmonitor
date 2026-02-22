package main

import (
	"fmt"
	net1 "net"
	"sort"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	// dnsCache stores IP -> domain name mappings.
	dnsCache = struct {
		sync.RWMutex
		m map[string]string
	}{m: make(map[string]string)}

	// lookupQueue is a channel for IPs that need DNS resolution.
	lookupQueue = make(chan string, 256)
)

// dnsResolver performs reverse DNS lookups for IPs from the lookupQueue
// and updates the cache.
func dnsResolver() {
	for ip := range lookupQueue {
		names, err := net1.LookupAddr(ip)
		var name string
		if err != nil || len(names) == 0 {
			name = ip
		} else {
			name = strings.TrimSuffix(names[0], ".")
		}

		dnsCache.Lock()
		dnsCache.m[ip] = name
		dnsCache.Unlock()
	}
}

// fetchSystemInfo retrieves CPU and Memory statistics.
func fetchSystemInfo() (float64, *mem.VirtualMemoryStat, error) {
	cpuPercentages, err := cpu.Percent(0, false)
	if err != nil {
		return 0, nil, err
	}
	cpuUsage := 0.0
	if len(cpuPercentages) > 0 {
		cpuUsage = cpuPercentages[0]
	}

	vm, err := mem.VirtualMemory()
	return cpuUsage, vm, err
}

// fetchNetworkInfo retrieves network I/O statistics.
func fetchNetworkInfo(prevRecv, prevSent uint64) (float64, float64, uint64, uint64, error) {
	currentNetStats, err := net.IOCounters(false)
	if err != nil || len(currentNetStats) == 0 {
		return 0, 0, prevRecv, prevSent, err
	}

	dlSpeed := float64(currentNetStats[0].BytesRecv-prevRecv) / 1024 / 2 // KB/s
	ulSpeed := float64(currentNetStats[0].BytesSent-prevSent) / 1024 / 2
	newRecv := currentNetStats[0].BytesRecv
	newSent := currentNetStats[0].BytesSent

	return dlSpeed, ulSpeed, newRecv, newSent, nil
}

// fetchProcessList retrieves and sorts the list of running processes.
func fetchProcessList(v *mem.VirtualMemoryStat) ([]ProcessInfo, float64, map[int32]string, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, 0, nil, err
	}

	var procList []ProcessInfo
	var totalProcCPU float64
	pidToName := make(map[int32]string)

	for _, p := range processes {
		name, _ := p.Name()
		pidToName[p.Pid] = name
		memPercent, _ := p.MemoryPercent()
		cpuPercent, _ := p.CPUPercent()

		if memPercent > 0.1 || cpuPercent > 0.1 {
			procList = append(procList, ProcessInfo{
				PID:  p.Pid,
				Name: name,
				CPU:  cpuPercent,
				Mem:  memPercent,
			})
			totalProcCPU += cpuPercent
		}
	}

	sort.Slice(procList, func(i, j int) bool {
		return procList[i].Mem > procList[j].Mem
	})

	return procList, totalProcCPU, pidToName, nil
}

// fetchConnectionList retrieves and sorts the list of network connections.
func fetchConnectionList(pidToName map[int32]string) ([]ConnInfo, error) {
	var connList []ConnInfo
	connections, err := net.Connections("inet")
	if err != nil {
		return nil, err
	}

	for _, conn := range connections {
		if conn.Status == "LISTEN" || conn.Status == "NONE" || conn.Pid == 0 || len(conn.Raddr.IP) == 0 || conn.Raddr.IP == "127.0.0.1" || conn.Raddr.IP == "::1" {
			continue
		}

		procName, ok := pidToName[conn.Pid]
		if !ok {
			procName = "N/A"
		}

		remoteDisplay := getDNS(conn.Raddr.IP)
		localAddr := fmt.Sprintf("%s:%d", conn.Laddr.IP, conn.Laddr.Port)
		remoteAddrWithPort := fmt.Sprintf("%s:%d", remoteDisplay, conn.Raddr.Port)

		connList = append(connList, ConnInfo{
			PID:         conn.Pid,
			ProcessName: procName,
			LocalAddr:   localAddr,
			RemoteAddr:  remoteAddrWithPort,
			Status:      conn.Status,
		})
	}

	sort.Slice(connList, func(i, j int) bool {
		iEst := connList[i].Status == "ESTABLISHED"
		jEst := connList[j].Status == "ESTABLISHED"
		if iEst != jEst {
			return iEst
		}
		if connList[i].ProcessName != connList[j].ProcessName {
			return connList[i].ProcessName < connList[j].ProcessName
		}
		return connList[i].PID < connList[j].PID
	})

	return connList, nil
}

// getDNS performs a reverse DNS lookup, utilizing a cache.
func getDNS(ip string) string {
	dnsCache.RLock()
	name, found := dnsCache.m[ip]
	dnsCache.RUnlock()

	if found {
		return name
	}

	dnsCache.Lock()
	if _, exists := dnsCache.m[ip]; !exists {
		dnsCache.m[ip] = ip // Placeholder
		select {
		case lookupQueue <- ip:
		default:
		}
	}
	dnsCache.Unlock()
	return ip
}
