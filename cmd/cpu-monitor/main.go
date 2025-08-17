package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-autostart"
	"github.com/getlantern/systray"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// --- Configuration ---

type Config struct {
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	DefaultDisplay         string `json:"default_display"`
	DiskDevice             string `json:"disk_device"` // e.g., "sda" on Linux, "C:" on Windows
	NetworkInterface       string `json:"network_interface"`
	AutoStart              bool   `json:"auto_start"`
}

var config Config
var configPath string
var logPath string
var autostartApp *autostart.App

func initPaths() {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("Could not find user config directory: %v", err)
	}
	appConfigDir := filepath.Join(userConfigDir, "CPU-Monitor")
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		log.Fatalf("Could not create app config directory: %v", err)
	}
	configPath = filepath.Join(appConfigDir, "config.json")
	logPath = filepath.Join(appConfigDir, "cpu-monitor.log")
}

func loadConfig() {
	// Default values
	config = Config{
		RefreshIntervalSeconds: 2,
		DefaultDisplay:         "cpu_ram",
		DiskDevice:             "", // Empty means all devices
		NetworkInterface:       "", // Empty means first interface
		AutoStart:              false,
	}

	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, create it with default values
		saveConfig()
		return
	}

	json.Unmarshal(file, &config)
}

func saveConfig() {
	file, _ := json.MarshalIndent(config, "", "  ")
	err := ioutil.WriteFile(configPath, file, 0644)
	if err != nil {
		log.Printf("Error saving config: %v", err)
	}
}

// --- Application State ---

// DisplayState represents the metric currently shown in the systray.
type DisplayState int

const (
	ShowCPUAndRAM DisplayState = iota
	ShowDisk
	ShowNetwork
)

var currentDisplayState DisplayState
var refreshInterval time.Duration

func main() {
	initPaths()
	// Set up a log file for debugging
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	systray.Run(onReady, onExit)
}

func onReady() {
	loadConfig()

	// Autostart configuration
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Could not get executable path: %v", err)
	}
	autostartApp = &autostart.App{
		Name:        "cpu-monitor",
		DisplayName: "CPU Monitor",
		Exec:        []string{execPath},
	}

	// Set initial state from config
	refreshInterval = time.Duration(config.RefreshIntervalSeconds) * time.Second
	switch config.DefaultDisplay {
	case "disk":
		currentDisplayState = ShowDisk
	case "network":
		currentDisplayState = ShowNetwork
	default:
		currentDisplayState = ShowCPUAndRAM
	}

	systray.SetIcon(IconData)
	systray.SetTitle("Loading...")
	systray.SetTooltip("Fetching system metrics...")

	// --- Display Submenu ---
	mDisplay := systray.AddMenuItem("Display", "Select which metric to display")
	mShowCPUAndRAM := mDisplay.AddSubMenuItem("CPU and RAM", "Show CPU and RAM usage")
	mShowDisk := mDisplay.AddSubMenuItem("Disk I/O", "Show disk read/write speed")
	mShowNetwork := mDisplay.AddSubMenuItem("Network I/O", "Show network up/down speed")

	// --- Refresh Rate Submenu ---
	mRefreshRate := systray.AddMenuItem("Refresh Rate", "Set the refresh interval")
	mRefresh1s := mRefreshRate.AddSubMenuItem("1 Second", "Refresh every second")
	mRefresh2s := mRefreshRate.AddSubMenuItem("2 Seconds", "Refresh every 2 seconds")
	mRefresh5s := mRefreshRate.AddSubMenuItem("5 Seconds", "Refresh every 5 seconds")

	// --- Settings Submenu ---
	mSettings := systray.AddMenuItem("Settings", "Application settings")
	mAutoStart := mSettings.AddSubMenuItem("Launch at Startup", "Enable or disable launching the app at system startup")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	// Set initial check state for autostart
	if config.AutoStart {
		mAutoStart.Check()
	}

	// Goroutine to handle menu clicks and update config
	go func() {
		for {
			select {
			// Display state changes
			case <-mShowCPUAndRAM.ClickedCh:
				currentDisplayState = ShowCPUAndRAM
				config.DefaultDisplay = "cpu_ram"
				saveConfig()
			case <-mShowDisk.ClickedCh:
				currentDisplayState = ShowDisk
				config.DefaultDisplay = "disk"
				saveConfig()
			case <-mShowNetwork.ClickedCh:
				currentDisplayState = ShowNetwork
				config.DefaultDisplay = "network"
				saveConfig()

			// Refresh interval changes
			case <-mRefresh1s.ClickedCh:
				refreshInterval = 1 * time.Second
				config.RefreshIntervalSeconds = 1
				saveConfig()
			case <-mRefresh2s.ClickedCh:
				refreshInterval = 2 * time.Second
				config.RefreshIntervalSeconds = 2
				saveConfig()
			case <-mRefresh5s.ClickedCh:
				refreshInterval = 5 * time.Second
				config.RefreshIntervalSeconds = 5
				saveConfig()

			// Settings changes
			case <-mAutoStart.ClickedCh:
				if mAutoStart.Checked() {
					// It's currently checked, so we're disabling it
					if err := autostartApp.Disable(); err != nil {
						log.Printf("Failed to disable autostart: %v", err)
					} else {
						mAutoStart.Uncheck()
						config.AutoStart = false
						saveConfig()
						log.Println("Disabled autostart")
					}
				} else {
					// It's currently unchecked, so we're enabling it
					if err := autostartApp.Enable(); err != nil {
						log.Printf("Failed to enable autostart: %v", err)
					} else {
						mAutoStart.Check()
						config.AutoStart = true
						saveConfig()
						log.Println("Enabled autostart")
					}
				}

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	// Goroutine to update metrics
	go func() {
		var lastNetStats map[string]net.IOCountersStat
		var lastDiskStats map[string]disk.IOCountersStat
		var lastTime time.Time

		// Initial data fetch
		lastNetStats = make(map[string]net.IOCountersStat)
		netStats, err := net.IOCounters(true)
		if err == nil {
			for _, s := range netStats {
				lastNetStats[s.Name] = s
			}
		}
		lastDiskStats, _ = disk.IOCounters() // Prime the pump
		lastTime = time.Now()

		for {
			// --- Data Collection ---
			cpuPercent, _ := cpu.Percent(0, false) // Use 0 for non-blocking
			vm, _ := mem.VirtualMemory()

			now := time.Now()
			duration := now.Sub(lastTime).Seconds()
			if duration <= 0 {
				duration = 1 // Avoid division by zero on first run
			}

			// Network speed
			currentNetStatsSlice, err := net.IOCounters(true)
			if err != nil {
				log.Printf("Error getting network IO counters: %v", err)
			}
			currentNetStats := make(map[string]net.IOCountersStat)
			var interfaceNames []string
			for _, s := range currentNetStatsSlice {
				currentNetStats[s.Name] = s
				interfaceNames = append(interfaceNames, s.Name)
			}
			log.Printf("Found network interfaces: %v", interfaceNames)

			var netReadSpeed, netWriteSpeed float64
			var targetInterfaceName string

			if config.NetworkInterface != "" {
				targetInterfaceName = config.NetworkInterface
			} else {
				// Find the interface with the most traffic (likely the primary one)
				var maxBytesSent uint64 = 0
				for _, s := range currentNetStatsSlice {
					if !strings.HasPrefix(s.Name, "lo") && s.BytesSent > maxBytesSent {
						maxBytesSent = s.BytesSent
						targetInterfaceName = s.Name
					}
				}
			}
			log.Printf("Target network interface: %s", targetInterfaceName)

			if current, ok := currentNetStats[targetInterfaceName]; ok {
				if last, ok := lastNetStats[targetInterfaceName]; ok {
					netReadSpeed = float64(current.BytesRecv-last.BytesRecv) / duration
					netWriteSpeed = float64(current.BytesSent-last.BytesSent) / duration
					log.Printf(
						"Calculating speed for %s: Current (Recv: %d, Sent: %d), Last (Recv: %d, Sent: %d), Duration: %.2f",
						targetInterfaceName, current.BytesRecv, current.BytesSent, last.BytesRecv, last.BytesSent, duration,
					)
				} else {
					log.Printf("No previous stats found for interface: %s", targetInterfaceName)
				}
			} else {
				log.Printf("Target interface not found in current stats: %s", targetInterfaceName)
			}
			lastNetStats = currentNetStats

			// Disk speed
			currentDiskStats, err := disk.IOCounters()
			if err != nil {
				log.Printf("Error getting disk IO counters: %v", err)
			}

			var totalReadBytes, totalWriteBytes uint64
			var deviceNames []string
			for name, current := range currentDiskStats {
				deviceNames = append(deviceNames, name)
				if last, ok := lastDiskStats[name]; ok {
					totalReadBytes += current.ReadBytes - last.ReadBytes
					totalWriteBytes += current.WriteBytes - last.WriteBytes
				}
			}
			log.Printf("Found disk devices for IO stats: %v", deviceNames)

			diskReadSpeed := float64(totalReadBytes) / duration
			diskWriteSpeed := float64(totalWriteBytes) / duration
			lastDiskStats = currentDiskStats
			lastTime = now

			// --- UI Update ---
			var title, tooltip string
			switch currentDisplayState {
			case ShowCPUAndRAM:
				title = fmt.Sprintf("CPU: %.2f%%, RAM: %.2f%%", cpuPercent[0], vm.UsedPercent)
				tooltip = title
			case ShowDisk:
				title = fmt.Sprintf("Disk R: %s, W: %s", formatSpeed(diskReadSpeed), formatSpeed(diskWriteSpeed))
				tooltip = fmt.Sprintf("Disk Read: %s/s\nDisk Write: %s/s", formatSpeed(diskReadSpeed), formatSpeed(diskWriteSpeed))
			case ShowNetwork:
				title = fmt.Sprintf("Net D: %s, U: %s", formatSpeed(netReadSpeed), formatSpeed(netWriteSpeed))
				tooltip = fmt.Sprintf("Network Down: %s/s\nNetwork Up: %s/s", formatSpeed(netReadSpeed), formatSpeed(netWriteSpeed))
			}
			systray.SetTitle(title)
			systray.SetTooltip(tooltip)

			time.Sleep(refreshInterval)
		}
	}()
}

func onExit() {
	// No special cleanup needed
}

func formatSpeed(bytesPerSecond float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GB/s", bytesPerSecond/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.2f B/s", bytesPerSecond)
	}
}
