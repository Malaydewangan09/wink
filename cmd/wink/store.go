package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusDead    Status = "dead"
)

type Service struct {
	Name      string    `json:"name"`
	Cmd       string    `json:"cmd"`
	PID       int       `json:"pid"`
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
}

type LogLine struct {
	Service   string    `json:"service"`
	Text      string    `json:"text"`
	Stream    string    `json:"stream"` // stdout or stderr
	Timestamp time.Time `json:"ts"`
}

func winkDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wink")
}

func servicesPath() string {
	return filepath.Join(winkDir(), "services.json")
}

func logsPath() string {
	return filepath.Join(winkDir(), "logs.jsonl")
}

func ensureDir() error {
	return os.MkdirAll(winkDir(), 0755)
}

// statusFromErr determines whether a process exit was intentional or a crash.
// SIGTERM/SIGINT/SIGKILL = user stopped it = StatusStopped (no notification).
// Non-zero exit code = unexpected crash = StatusDead (triggers notification).
func statusFromErr(err error) Status {
	if err == nil {
		return StatusStopped
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			sig := status.Signal()
			if sig == syscall.SIGTERM || sig == syscall.SIGINT || sig == syscall.SIGKILL {
				return StatusStopped
			}
		}
	}
	return StatusDead
}

// updateService locks services.json, loads it fresh, applies fn, then saves.
// Use this from collectors to avoid race conditions between concurrent processes.
func updateService(name string, fn func(map[string]*Service)) error {
	_ = ensureDir()
	lockPath := servicesPath() + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint
	services, _ := loadServices()
	fn(services)
	return saveServices(services)
}

func loadServices() (map[string]*Service, error) {
	if err := ensureDir(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(servicesPath())
	if os.IsNotExist(err) {
		return map[string]*Service{}, nil
	}
	if err != nil {
		return nil, err
	}
	var services map[string]*Service
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, err
	}
	return services, nil
}

func saveServices(services map[string]*Service) error {
	if err := ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(servicesPath(), data, 0644)
}

var appendCounter int64

func appendLog(line LogLine) error {
	if err := ensureDir(); err != nil {
		return err
	}
	f, err := os.OpenFile(logsPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(f, "%s\n", data); err != nil {
		return err
	}
	// trim every 100 appends to keep logs.jsonl bounded
	if atomic.AddInt64(&appendCounter, 1)%100 == 0 {
		_ = trimLogs()
	}
	return nil
}

// trimLogs removes the oldest lines when total exceeds max_log_lines from config.
func trimLogs() error {
	cfg, _ := loadConfig()
	if cfg == nil || cfg.MaxLogLines <= 0 {
		return nil
	}
	lines, err := readLogs("")
	if err != nil || len(lines) <= cfg.MaxLogLines {
		return nil
	}
	lines = lines[len(lines)-cfg.MaxLogLines:]
	f, err := os.Create(logsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, l := range lines {
		_ = enc.Encode(l)
	}
	return nil
}

func readLogs(filter string) ([]LogLine, error) {
	data, err := os.ReadFile(logsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var lines []LogLine
	for _, raw := range splitLines(string(data)) {
		if raw == "" {
			continue
		}
		var l LogLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			continue
		}
		if filter == "" || l.Service == filter {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
