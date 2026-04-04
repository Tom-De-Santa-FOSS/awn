package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runDaemon(args []string, addr string, caller rpcCaller) (string, error) {
	sub := args[0]
	switch sub {
	case "start":
		return daemonStart(addr)
	case "stop":
		return daemonStop()
	case "status":
		return daemonStatus(addr, caller)
	default:
		return "", fmt.Errorf("unknown daemon subcommand: %s\n\nUsage: awn daemon <start|stop|status>", sub)
	}
}

func pidFilePath() string {
	dir := os.Getenv("AWN_STATE_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache", "awn")
	}
	return filepath.Join(dir, "awnd.pid")
}

func daemonStart(addr string) (string, error) {
	// Check if already running
	host := strings.TrimPrefix(addr, "ws://")
	if conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond); err == nil {
		conn.Close()
		return "daemon already running\n", nil
	}

	// Find awnd binary
	awndPath, err := exec.LookPath("awnd")
	if err != nil {
		return "", fmt.Errorf("awnd not found in PATH: %w", err)
	}

	cmd := exec.Command(awndPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start awnd: %w", err)
	}

	// Write PID file
	pidPath := pidFilePath()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err == nil {
		_ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	}

	// Release the process so it outlives us
	_ = cmd.Process.Release()

	// Poll for readiness
	for i := range 50 {
		_ = i
		time.Sleep(100 * time.Millisecond)
		if conn, err := net.DialTimeout("tcp", host, 200*time.Millisecond); err == nil {
			conn.Close()
			return fmt.Sprintf("daemon started (pid %d)\n", cmd.Process.Pid), nil
		}
	}

	return "", fmt.Errorf("daemon started but not reachable after 5s")
}

func daemonStop() (string, error) {
	pidPath := pidFilePath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return "", fmt.Errorf("no PID file found (is the daemon running?): %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return "", fmt.Errorf("invalid PID in %s: %w", pidPath, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return "", fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := proc.Signal(os.Interrupt); err != nil {
		// Process might already be dead
		_ = os.Remove(pidPath)
		return "daemon not running (stale PID file removed)\n", nil
	}

	_ = os.Remove(pidPath)
	return fmt.Sprintf("daemon stopped (pid %d)\n", pid), nil
}

func daemonStatus(addr string, caller rpcCaller) (string, error) {
	result, err := caller(addr, "ping", nil)
	if err != nil {
		return "daemon not running\n", nil
	}
	return fmt.Sprintf("daemon running: %s\n", result), nil
}
