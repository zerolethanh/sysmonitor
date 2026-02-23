package main

import (
	"flag"
	"fmt"
	net1 "net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/net"
)

func main() {
	go dnsResolver()
	var procLimit int
	flag.IntVar(&procLimit, "limit", 100, "Maximum number of processes to display in the table")
	flag.Parse()

	app := tview.NewApplication()

	// Initialize UI components from ui.go
	sysInfoView := createSysInfoView()
	netView := createNetView()
	procTable := createProcTable()
	netConnTable := createNetConnTable()

	// Layout setup
	bottomFlex := tview.NewFlex().
		AddItem(procTable, 0, 1, true).
		AddItem(netConnTable, 0, 1, true)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(netView, 3, 1, false).
		AddItem(sysInfoView, 3, 1, false).
		AddItem(bottomFlex, 0, 1, true)

	// Input capture
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			if procTable.HasFocus() {
				app.SetFocus(netConnTable)
			} else {
				app.SetFocus(procTable)
			}
			return nil
		}

		if netConnTable.HasFocus() {
			return listenKeyForNetConnTable(event, netConnTable)
		}
		if procTable.HasFocus() {
			return listenKeyForProcTable(event, procTable)
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

func listenKeyForProcTable(event *tcell.EventKey, procTable *tview.Table) *tcell.EventKey {
	// Setup event handlers
	procTable.SetSelectedFunc(func(row int, column int) {
		if row > 0 {
			pidStr := procTable.GetCell(row, 0).Text
			if pid, err := strconv.Atoi(pidStr); err == nil {
				runWitr(pid)
			}
		}
	})
	var focusedRow int
	if procTable.HasFocus() {
		focusedRow, _ = procTable.GetSelection()
		if focusedRow == 0 {
			// skip title
			return nil
		}
	}
	// Run `witr` for the selected process in the network table when 'w' is pressed
	if event.Rune() == 'w' {
		pidStr := procTable.GetCell(focusedRow, 0).Text
		pid, err := strconv.Atoi(pidStr)
		if err == nil {
			runWitr(pid)
		}
		return nil // Cancel the 'w' event
	}
	return event
}

func listenKeyForNetConnTable(event *tcell.EventKey, netConnTable *tview.Table) *tcell.EventKey {
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
	var focusedRow int
	if netConnTable.HasFocus() {
		focusedRow, _ = netConnTable.GetSelection()
		if focusedRow == 0 {
			// skip title
			return nil
		}
	}
	// Run `witr` for the selected process in the network table when 'w' is pressed
	if event.Rune() == 'w' {
		pidStr := netConnTable.GetCell(focusedRow, 0).Text
		pid, err := strconv.Atoi(pidStr)
		if err == nil {
			runWitr(pid)
		}
		return nil // Cancel the 'w' event
	}

	if event.Rune() == 'l' {
		localAddress := netConnTable.GetCell(focusedRow, 2).Text
		if localAddress != "" {
			runOpen(localAddress)
		}
		return nil
	}

	if event.Rune() == 'r' {
		removeAddress := netConnTable.GetCell(focusedRow, 3).Text
		if removeAddress != "" {
			runOpen(removeAddress)
		}

		return nil
	}

	return event
}

// runWitr executes the 'witr' command for a given PID.
func runWitr(pid int) {
	cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"witr --pid %d\"", pid)
	cmd := exec.Command("osascript", "-e", cmdString)
	_ = cmd.Start()
}

// runOpen opens the given address in the default browser.
func runOpen(address string) {
	is443 := strings.HasSuffix(address, ":443")
	var protocol = "http"
	if is443 {
		protocol = "https"
	}
	cmd := exec.Command("open", fmt.Sprintf("%s://%s", protocol, address))
	_ = cmd.Start()
}

// runWhois executes the 'whois' command for a given address.
func runWhois(addr string) {
	cmdString := fmt.Sprintf("tell app \"Terminal\" to do script \"whois %s\"", addr)
	cmd := exec.Command("osascript", "-e", cmdString)
	_ = cmd.Start()
}
