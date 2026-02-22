package main

import (
	"flag"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessInfo struct {
	PID  int32
	Name string
	CPU  float64
	Mem  float32
}

func main() {
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

	// 4. Sắp xếp Layout (Chia theo hàng dọc)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(netView, 3, 1, false).     // netView chiếm cố định 3 dòng
		AddItem(sysInfoView, 3, 1, false). // sysInfoView chiếm cố định 3 dòng
		AddItem(procTable, 0, 1, true)     // procTable chiếm toàn bộ không gian còn lại

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
			// Lấy phần trăm sử dụng CPU tổng thể.
			// Tham số đầu tiên `0` nghĩa là tính trung bình trên tất cả các CPU.
			// Tham số thứ hai `false` nghĩa là không tính cho mỗi CPU riêng lẻ.
			cpuPercentages, _ := cpu.Percent(0, false)
			var cpuUsage float64
			if len(cpuPercentages) > 0 {
				cpuUsage = cpuPercentages[0]
			}

			// --- Xử lý Mạng ---
			currentNetStats, _ := net.IOCounters(false)
			var dlSpeed, ulSpeed float64
			if len(currentNetStats) > 0 {
				dlSpeed = float64(currentNetStats[0].BytesRecv-prevRecv) / 1024 / 2 // KB/s
				ulSpeed = float64(currentNetStats[0].BytesSent-prevSent) / 1024 / 2

				prevRecv = currentNetStats[0].BytesRecv
				prevSent = currentNetStats[0].BytesSent
			}

			// --- Xử lý Tiến trình ---
			processes, _ := process.Processes()
			var procList []ProcessInfo
			var totalProcCPU float64

			for _, p := range processes {
				name, _ := p.Name()
				memPercent, _ := p.MemoryPercent()
				cpuPercent, _ := p.CPUPercent() // Lấy giá trị tức thời

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

			// --- Cập nhật Giao diện (Quan trọng: phải đưa vào QueueUpdateDraw để an toàn luồng) ---
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

				// Tiêu đề cột
				headers := []string{"PID", "TÊN TIẾN TRÌNH", "CPU (%)", "RAM (%) / MB"}
				for c, header := range headers {
					cell := tview.NewTableCell(header).
						SetTextColor(tcell.ColorYellow).
						SetSelectable(false).
						SetAlign(tview.AlignLeft)
					procTable.SetCell(0, c, cell)
				}

				// Giới hạn hiển thị các tiến trình để tránh lag TUI
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
			})
		}
	}()

	// 6. Chạy ứng dụng TUI
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
