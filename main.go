package main

import (
	"flag"
	"fmt"
	net1 "net"
	"os/exec"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/net"
)

func main() {
	go dnsResolver()
	var procLimit int
	flag.IntVar(&procLimit, "limit", 100, "Số lượng tiến trình tối đa hiển thị trong bảng")
	flag.Parse()

	app := tview.NewApplication()

	// Initialize UI components from ui.go
	sysInfoView := createSysInfoView()
	netView := createNetView()
	procTable := createProcTable()
	netConnTable := createNetConnTable()

	// Setup event handlers
	procTable.SetSelectedFunc(func(row int, column int) {
		if row > 0 {
			pidStr := procTable.GetCell(row, 0).Text
			if pid, err := strconv.Atoi(pidStr); err == nil {
				runWitr(pid)
			}
		}
	})

	netConnTable.SetSelectedFunc(func(row int, column int) {
		if row > 0 {
			remoteAddrWithPort := netConnTable.GetCell(row, 3).Text
			addr, _, err := net1.SplitHostPort(remoteAddrWithPort)
			if err != nil {
				addr = remoteAddrWithPort
			}
			runWhois(addr)
		}
	})

	// Layout setup
	bottomFlex := tview.NewFlex().
		AddItem(procTable, 0, 1, true).
		AddItem(netConnTable, 0, 1, true)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(netView, 3, 1, false).
		AddItem(sysInfoView, 3, 1, false).
		AddItem(bottomFlex, 0, 1, true)

	// Input capture for Tab and 'w' key
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			if procTable.HasFocus() {
				app.SetFocus(netConnTable)
			} else {
				app.SetFocus(procTable)
			}
			return nil
		}

		//if event.Rune() == 'w' && netConnTable.HasFocus() {
		//	row, _ := netConnTable.GetSelection()
		//	if row > 0 {
		//		pidStr := netConnTable.GetCell(row, 0).Text
		//		if pid, err := strconv.Atoi(pidStr); err == nil {
		//			runWitr(pid)
		//		}
		//	}
		//	return nil
		//}

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

		return event
	})

	// Goroutine for continuous data fetching and UI updates
	go func() {
		initialNetStats, _ := net.IOCounters(false)
		var prevRecv, prevSent uint64
		if len(initialNetStats) > 0 {
			prevRecv = initialNetStats[0].BytesRecv
			prevSent = initialNetStats[0].BytesSent
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			cpuUsage, vm, _ := fetchSystemInfo()
			dlSpeed, ulSpeed, newRecv, newSent, _ := fetchNetworkInfo(prevRecv, prevSent)
			prevRecv, prevSent = newRecv, newSent
			procList, totalProcCPU, pidToName, _ := fetchProcessList(vm)
			connList, _ := fetchConnectionList(pidToName)

			app.QueueUpdateDraw(func() {
				updateSysInfoView(sysInfoView, cpuUsage, vm)
				updateNetView(netView, dlSpeed, ulSpeed)
				updateProcTable(procTable, procList, totalProcCPU, cpuUsage, vm, procLimit)
				updateNetConnTable(netConnTable, connList, 50)
			})
		}
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}

// runWitr executes the 'witr' command for a given PID.
func runWitr(pid int) {
	cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"witr --pid %d\"", pid)
	cmd := exec.Command("osascript", "-e", cmdString)
	_ = cmd.Start()
}

// runWhois executes the 'whois' command for a given address.
func runWhois(addr string) {
	cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"whois %s\"", addr)
	cmd := exec.Command("osascript", "-e", cmdString)
	_ = cmd.Start()
}
