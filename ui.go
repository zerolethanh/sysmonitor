package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/mem"
)

// createSysInfoView initializes the system information TextView.
func createSysInfoView() *tview.TextView {
	sysInfoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("Đang thu thập dữ liệu hệ thống...")
	sysInfoView.SetBorder(true).SetTitle(" 📊 System Info ").SetTitleColor(tcell.ColorGreen)
	return sysInfoView
}

// createNetView initializes the network I/O TextView.
func createNetView() *tview.TextView {
	netView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Đang thu thập dữ liệu mạng...")
	netView.SetBorder(true).SetTitle(" 🌐 Network I/O ").SetTitleColor(tcell.ColorGreen)
	return netView
}

// createProcTable initializes the process table.
func createProcTable() *tview.Table {
	procTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	procTable.SetBorder(true).SetTitle(" ⚙️ Top Processes (RAM) ").SetTitleColor(tcell.ColorCadetBlue)
	return procTable
}

// createNetConnTable initializes the network connections table.
func createNetConnTable() *tview.Table {
	netConnTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	netConnTable.SetBorder(true).SetTitle(" 🔌 Network Connections ").SetTitleColor(tcell.ColorGreen)
	return netConnTable
}

// updateSysInfoView updates the system info view with the latest data.
func updateSysInfoView(view *tview.TextView, cpuUsage float64, v *mem.VirtualMemoryStat) {
	sysInfoText := fmt.Sprintf(
		"[yellow]CPU Usage: [white]%5.2f%%   [yellow]RAM (Used/Total): [white]%.2f/%.2f GiB (%5.2f%%)\n"+
			"             [yellow]Available: [white]%.2f GiB",
		cpuUsage,
		float64(v.Used)/1024/1024/1024,
		float64(v.Total)/1024/1024/1024,
		v.UsedPercent,
		float64(v.Available)/1024/1024/1024,
	)
	view.SetText(sysInfoText)
}

// updateNetView updates the network view with the latest data.
func updateNetView(view *tview.TextView, dlSpeed, ulSpeed float64) {
	timeStr := time.Now().Format("15:04:05")
	netText := fmt.Sprintf("[yellow]Tải xuống (In):[white] %7.2f KB/s   |   [yellow]Tải lên (Out):[white] %7.2f KB/s   |   🕒 %s", dlSpeed, ulSpeed, timeStr)
	view.SetText(netText)
}

// updateProcTable updates the process table with the latest data.
func updateProcTable(table *tview.Table, procList []ProcessInfo, totalProcCPU, cpuUsage float64, v *mem.VirtualMemoryStat, limit int) {
	table.Clear()
	headers := []string{"PID", "TÊN TIẾN TRÌNH", "CPU (%)", "RAM (%) / MB"}
	for c, header := range headers {
		table.SetCell(0, c, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false).SetAlign(tview.AlignLeft))
	}

	if len(procList) < limit {
		limit = len(procList)
	}
	for r := 0; r < limit; r++ {
		p := procList[r]
		var relativeCPU float64
		if totalProcCPU > 0 {
			relativeCPU = (p.CPU / totalProcCPU) * cpuUsage
		}
		ramUsedMB := (float64(p.Mem) / 100.0) * (float64(v.Total) / (1024 * 1024))
		table.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", p.PID)).SetTextColor(tcell.ColorWhite))
		table.SetCell(r+1, 1, tview.NewTableCell(p.Name).SetTextColor(tcell.ColorGreen))
		table.SetCell(r+1, 2, tview.NewTableCell(fmt.Sprintf("%.2f", relativeCPU)).SetTextColor(tcell.ColorWhite))
		table.SetCell(r+1, 3, tview.NewTableCell(fmt.Sprintf("%.2f%% / %.2fMB", p.Mem, ramUsedMB)).SetTextColor(tcell.ColorWhite))
	}
}

// updateNetConnTable updates the network connections table with the latest data.
func updateNetConnTable(table *tview.Table, connList []ConnInfo, limit int) {
	table.Clear()
	connHeaders := []string{"PID", "PROCESS", "LOCAL ADDR", "REMOTE ADDR", "STATUS"}
	for c, header := range connHeaders {
		table.SetCell(0, c, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false).SetAlign(tview.AlignLeft))
	}

	if len(connList) < limit {
		limit = len(connList)
	}
	for r := 0; r < limit; r++ {
		cInfo := connList[r]
		table.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", cInfo.PID)).SetTextColor(tcell.ColorWhite))
		table.SetCell(r+1, 1, tview.NewTableCell(cInfo.ProcessName).SetTextColor(tcell.ColorGreen))
		table.SetCell(r+1, 2, tview.NewTableCell(cInfo.LocalAddr).SetTextColor(tcell.ColorWhite))
		table.SetCell(r+1, 3, tview.NewTableCell(cInfo.RemoteAddr).SetTextColor(tcell.ColorWhite))
		table.SetCell(r+1, 4, tview.NewTableCell(cInfo.Status).SetTextColor(tcell.ColorCadetBlue))
	}
}
