package main

import (
	"flag"
	"fmt"
	net1 "net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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

type ProcessInfo struct {
	PID  int32
	Name string
	CPU  float64
	Mem  float32
}

// ConnInfo holds information about a network connection.
type ConnInfo struct {
	PID         int32
	ProcessName string
	LocalAddr   string
	RemoteAddr  string
	Status      string
}

// dnsResolver performs reverse DNS lookups for IPs from the lookupQueue
// and updates the cache.
func dnsResolver() {
	for ip := range lookupQueue {
		names, err := net1.LookupAddr(ip)
		var name string
		if err != nil || len(names) == 0 {
			// On failure or no result, cache the IP itself to prevent re-lookup.
			name = ip
		} else {
			// Success, cache the first name, removing the trailing dot.
			name = strings.TrimSuffix(names[0], ".")
		}

		dnsCache.Lock()
		dnsCache.m[ip] = name
		dnsCache.Unlock()
	}
}

func main() {
	go dnsResolver()
	// Khởi tạo biến để lưu trữ giới hạn tiến trình và phân tích tham số dòng lệnh
	var procLimit int
	flag.IntVar(&procLimit, "limit", 100, "Số lượng tiến trình tối đa hiển thị trong bảng")
	flag.Parse()

	// Khởi tạo ứng dụng TUI
	app := tview.NewApplication()

	// 1. Khung hiển thị thông tin hệ thống (CPU & RAM)
	sysInfoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("Đang thu thập dữ liệu hệ thống...")
	sysInfoView.SetBorder(true).SetTitle(" 📊 System Info ").SetTitleColor(tcell.ColorGreen)

	// 2. Khung hiển thị Mạng (Network View)
	netView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Đang thu thập dữ liệu mạng...")
	netView.SetBorder(true).SetTitle(" 🌐 Network I/O ").SetTitleColor(tcell.ColorGreen)

	// 3. Bảng hiển thị Tiến trình (Process Table)
	procTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false) // Cho phép dùng phím mũi tên lên/xuống để chọn dòng
	procTable.SetBorder(true).SetTitle(" ⚙️ Top Processes (RAM) ").SetTitleColor(tcell.ColorCadetBlue)

	// Xử lý sự kiện khi nhấn Enter trên một dòng
	procTable.SetSelectedFunc(func(row int, column int) {
		// Bỏ qua dòng tiêu đề
		if row == 0 {
			return
		}
		pidStr := procTable.GetCell(row, 0).Text
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			// Bỏ qua nếu không phải là số
			return
		}

		// Lệnh cho macOS để mở cửa sổ Terminal mới và chạy 'witr'
		cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"witr --pid %d\"", pid)
		cmd := exec.Command("osascript", "-e", cmdString)

		// Thực thi lệnh mà không chờ (fire-and-forget)
		_ = cmd.Start()
	})

	// 3.5 Bảng kết nối mạng (Network Connection Table)
	netConnTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	netConnTable.SetBorder(true).SetTitle(" 🔌 Network Connections ").SetTitleColor(tcell.ColorGreen)

	// Xử lý sự kiện khi nhấn Enter trên một dòng của bảng kết nối mạng
	netConnTable.SetSelectedFunc(func(row int, column int) {
		// Bỏ qua dòng tiêu đề
		if row == 0 {
			return
		}
		// Lấy địa chỉ Remote Addr từ cột thứ 4 (index 3)
		remoteAddrWithPort := netConnTable.GetCell(row, 3).Text
		// Tách địa chỉ IP/domain ra khỏi port
		addr, _, err := net1.SplitHostPort(remoteAddrWithPort)
		if err != nil {
			// Nếu có lỗi (ví dụ không có port), dùng luôn chuỗi gốc
			addr = remoteAddrWithPort
		}

		// Lệnh cho macOS để mở cửa sổ Terminal mới và chạy 'whois'
		cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"whois %s\"", addr)
		cmd := exec.Command("osascript", "-e", cmdString)

		// Thực thi lệnh mà không chờ (fire-and-forget)
		_ = cmd.Start()
	})

	// 4. Sắp xếp Layout (Chia theo hàng dọc)
	bottomFlex := tview.NewFlex().
		AddItem(procTable, 0, 1, true).
		AddItem(netConnTable, 0, 1, true)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(netView, 3, 1, false).     // netView chiếm cố định 3 dòng
		AddItem(sysInfoView, 3, 1, false). // sysInfoView chiếm cố định 3 dòng
		AddItem(bottomFlex, 0, 1, true)    // bottomFlex chiếm toàn bộ không gian còn lại

	// Xử lý sự kiện nhấn phím để chuyển focus hoặc thực thi hành động
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Chuyển focus bằng phím Tab
		if event.Key() == tcell.KeyTab {
			if procTable.HasFocus() {
				app.SetFocus(netConnTable)
			} else {
				app.SetFocus(procTable)
			}
			return nil // Hủy sự kiện Tab mặc định
		}

		// Chạy `witr` cho tiến trình được chọn trong bảng network khi nhấn 'w'
		if event.Rune() == 'w' {
			if netConnTable.HasFocus() {
				row, _ := netConnTable.GetSelection()
				if row > 0 { // Bỏ qua dòng tiêu đề
					pidStr := netConnTable.GetCell(row, 0).Text
					pid, err := strconv.Atoi(pidStr)
					if err == nil {
						cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"witr --pid %d\"", pid)
						cmd := exec.Command("osascript", "-e", cmdString)
						_ = cmd.Start()
					}
				}
				return nil // Hủy sự kiện 'w'
			}
		}

		return event // Trả về sự kiện cho các xử lý khác
	})

	// 5. Goroutine chạy ngầm để lấy dữ liệu liên tục
	go func() {
		// Khởi tạo mốc mạng ban đầu
		initialNetStats, _ := net.IOCounters(false)
		var prevRecv, prevSent uint64
		if len(initialNetStats) > 0 {
			prevRecv = initialNetStats[0].BytesRecv
			prevSent = initialNetStats[0].BytesSent
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// --- Xử lý hệ thống (CPU & RAM) ---
			v, _ := mem.VirtualMemory()
			cpuPercentages, _ := cpu.Percent(0, false)
			var cpuUsage float64
			if len(cpuPercentages) > 0 {
				cpuUsage = cpuPercentages[0]
			}

			// --- Xử lý Mạng (Tổng quan) ---
			currentNetStats, _ := net.IOCounters(false)
			var dlSpeed, ulSpeed float64
			if len(currentNetStats) > 0 {
				dlSpeed = float64(currentNetStats[0].BytesRecv-prevRecv) / 1024 / 2 // KB/s
				ulSpeed = float64(currentNetStats[0].BytesSent-prevSent) / 1024 / 2
				prevRecv = currentNetStats[0].BytesRecv
				prevSent = currentNetStats[0].BytesSent
			}

			// --- Xử lý Tiến trình và tạo map PID -> Tên ---
			processes, _ := process.Processes()
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

			// Sắp xếp theo RAM giảm dần
			sort.Slice(procList, func(i, j int) bool {
				return procList[i].Mem > procList[j].Mem
			})

			// --- Xử lý Kết nối mạng (Chi tiết) ---
			var connList []ConnInfo
			connections, _ := net.Connections("inet")
			for _, conn := range connections {
				// Bỏ qua các kết nối không liên quan
				if conn.Status == "LISTEN" || conn.Status == "NONE" || conn.Pid == 0 || len(conn.Raddr.IP) == 0 {
					continue
				}
				// Bỏ qua các kết nối localhost
				if conn.Raddr.IP == "127.0.0.1" || conn.Raddr.IP == "::1" {
					continue
				}

				procName, ok := pidToName[conn.Pid]
				if !ok {
					procName = "N/A"
				}

				// --- Reverse DNS Lookup ---
				remoteIP := conn.Raddr.IP
				var remoteDisplay string

				dnsCache.RLock()
				name, found := dnsCache.m[remoteIP]
				dnsCache.RUnlock()

				if found {
					remoteDisplay = name // Use cached name (or IP if lookup failed)
				} else {
					remoteDisplay = remoteIP // Use IP for now
					// Add to queue for lookup, non-blocking.
					// Put a placeholder in cache to prevent re-queueing.
					dnsCache.Lock()
					if _, exists := dnsCache.m[remoteIP]; !exists {
						dnsCache.m[remoteIP] = remoteIP // Use IP as placeholder
						select {
						case lookupQueue <- remoteIP:
						default: // a non-blocking send
						}
					}
					dnsCache.Unlock()
				}

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

			// Sắp xếp danh sách kết nối: ESTABLISHED lên đầu, sau đó theo tên tiến trình
			sort.Slice(connList, func(i, j int) bool {
				// Ưu tiên trạng thái "ESTABLISHED"
				iEst := connList[i].Status == "ESTABLISHED"
				jEst := connList[j].Status == "ESTABLISHED"
				if iEst != jEst {
					return iEst // true (ESTABLISHED) sẽ được đưa lên đầu
				}
				// Sắp xếp theo tên tiến trình
				if connList[i].ProcessName != connList[j].ProcessName {
					return connList[i].ProcessName < connList[j].ProcessName
				}
				// Cuối cùng, sắp xếp theo PID để ổn định
				return connList[i].PID < connList[j].PID
			})

			// --- Cập nhật Giao diện ---
			app.QueueUpdateDraw(func() {
				// Update Text thông tin hệ thống
				sysInfoText := fmt.Sprintf(
					"[yellow]CPU Usage: [white]%5.2f%%   [yellow]RAM (Used/Total): [white]%.2f/%.2f GiB (%5.2f%%)\n"+
						"             [yellow]Available: [white]%.2f GiB",
					cpuUsage,
					float64(v.Used)/1024/1024/1024,
					float64(v.Total)/1024/1024/1024,
					v.UsedPercent,
					float64(v.Available)/1024/1024/1024,
				)
				sysInfoView.SetText(sysInfoText)

				// Update Text Mạng
				timeStr := time.Now().Format("15:04:05")
				netText := fmt.Sprintf("[yellow]Tải xuống (In):[white] %7.2f KB/s   |   [yellow]Tải lên (Out):[white] %7.2f KB/s   |   🕒 %s", dlSpeed, ulSpeed, timeStr)
				netView.SetText(netText)

				// Update Bảng Tiến trình
				procTable.Clear()
				headers := []string{"PID", "TÊN TIẾN TRÌNH", "CPU (%)", "RAM (%) / MB"}
				for c, header := range headers {
					procTable.SetCell(0, c, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false).SetAlign(tview.AlignLeft))
				}
				limit := procLimit
				if len(procList) < limit {
					limit = len(procList)
				}
				for r := 0; r < limit; r++ {
					p := procList[r]
					var relativeCPU float64
					if totalProcCPU > 0 {
						relativeCPU = (p.CPU / totalProcCPU) * cpuUsage
					}
					procTable.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", p.PID)).SetTextColor(tcell.ColorWhite))
					procTable.SetCell(r+1, 1, tview.NewTableCell(p.Name).SetTextColor(tcell.ColorGreen))
					procTable.SetCell(r+1, 2, tview.NewTableCell(fmt.Sprintf("%.2f", relativeCPU)).SetTextColor(tcell.ColorWhite))
					ramUsedMB := (float64(p.Mem) / 100.0) * (float64(v.Total) / (1024 * 1024))
					procTable.SetCell(r+1, 3, tview.NewTableCell(fmt.Sprintf("%.2f%% / %.2fMB", p.Mem, ramUsedMB)).SetTextColor(tcell.ColorWhite))
				}

				// Update Bảng Kết nối mạng
				netConnTable.Clear()
				connHeaders := []string{"PID", "PROCESS", "LOCAL ADDR", "REMOTE ADDR", "STATUS"}
				for c, header := range connHeaders {
					netConnTable.SetCell(0, c, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false).SetAlign(tview.AlignLeft))
				}
				connLimit := 50
				if len(connList) < connLimit {
					connLimit = len(connList)
				}
				for r := 0; r < connLimit; r++ {
					cInfo := connList[r]
					netConnTable.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", cInfo.PID)).SetTextColor(tcell.ColorWhite))
					netConnTable.SetCell(r+1, 1, tview.NewTableCell(cInfo.ProcessName).SetTextColor(tcell.ColorGreen))
					netConnTable.SetCell(r+1, 2, tview.NewTableCell(cInfo.LocalAddr).SetTextColor(tcell.ColorWhite))
					netConnTable.SetCell(r+1, 3, tview.NewTableCell(cInfo.RemoteAddr).SetTextColor(tcell.ColorWhite))
					netConnTable.SetCell(r+1, 4, tview.NewTableCell(cInfo.Status).SetTextColor(tcell.ColorCadetBlue))
				}
			})
		}
	}()

	// 6. Chạy ứng dụng TUI
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
