package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
	// Khởi tạo ứng dụng TUI
	app := tview.NewApplication()

	// 1. Khung hiển thị Mạng (Network View)
	netView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Đang thu thập dữ liệu mạng...")
	netView.SetBorder(true).SetTitle(" 🌐 Network I/O ").SetTitleColor(tcell.ColorGreen)

	// 2. Bảng hiển thị Tiến trình (Process Table)
	procTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false) // Cho phép dùng phím mũi tên lên/xuống để chọn dòng
	procTable.SetBorder(true).SetTitle(" ⚙️ Top Processes (RAM) ").SetTitleColor(tcell.ColorCadetBlue)

	// 3. Sắp xếp Layout (Chia theo hàng dọc)
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(netView, 3, 1, false). // netView chiếm cố định 3 dòng
		AddItem(procTable, 0, 1, true) // procTable chiếm toàn bộ không gian còn lại

	// 4. Goroutine chạy ngầm để lấy dữ liệu liên tục
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
				}
			}

			// Sắp xếp theo RAM giảm dần
			sort.Slice(procList, func(i, j int) bool {
				return procList[i].Mem > procList[j].Mem
			})

			// --- Cập nhật Giao diện (Quan trọng: phải đưa vào QueueUpdateDraw để an toàn luồng) ---
			app.QueueUpdateDraw(func() {
				// Update Text Mạng
				timeStr := time.Now().Format("15:04:05")
				netText := fmt.Sprintf("[yellow]Tải xuống (In):[white] %7.2f KB/s   |   [yellow]Tải lên (Out):[white] %7.2f KB/s   |   🕒 %s", dlSpeed, ulSpeed, timeStr)
				netView.SetText(netText)

				// Update Bảng Tiến trình
				procTable.Clear()

				// Tiêu đề cột
				headers := []string{"PID", "TÊN TIẾN TRÌNH", "CPU (%)", "RAM (%)"}
				for c, header := range headers {
					cell := tview.NewTableCell(header).
						SetTextColor(tcell.ColorYellow).
						SetSelectable(false).
						SetAlign(tview.AlignLeft)
					procTable.SetCell(0, c, cell)
				}

				// Giới hạn hiển thị 20 tiến trình đầu tiên để tránh lag TUI
				limit := 20
				if len(procList) < limit {
					limit = len(procList)
				}

				for r := 0; r < limit; r++ {
					p := procList[r]

					procTable.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", p.PID)).SetTextColor(tcell.ColorWhite))
					procTable.SetCell(r+1, 1, tview.NewTableCell(p.Name).SetTextColor(tcell.ColorGreen))
					procTable.SetCell(r+1, 2, tview.NewTableCell(fmt.Sprintf("%.2f", p.CPU)).SetTextColor(tcell.ColorWhite))
					procTable.SetCell(r+1, 3, tview.NewTableCell(fmt.Sprintf("%.2f", p.Mem)).SetTextColor(tcell.ColorWhite))
				}
			})
		}
	}()

	// 5. Chạy ứng dụng TUI
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
