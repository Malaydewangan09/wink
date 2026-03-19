package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func cmdWatch(name string, cmdArgs []string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	if svc, exists := services[name]; exists && svc.Status == StatusRunning {
		fatal(fmt.Errorf("service %q is already running (pid %d)", name, svc.PID))
	}

	// self-exe as background collector daemon
	self, err := os.Executable()
	if err != nil {
		fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	collectorArgs := append([]string{"__collect", name}, cmdArgs...)
	collector := exec.Command(self, collectorArgs...)
	collector.Dir = cwd
	collector.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// redirect collector stderr to a temp log for debugging
	if err := ensureDir(); err != nil {
		fatal(err)
	}
	errLog, _ := os.OpenFile(winkDir()+"/collector-err.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	collector.Stdout = nil
	collector.Stderr = errLog
	collector.Stdin = nil

	if err := collector.Start(); err != nil {
		fatal(fmt.Errorf("failed to start collector: %w", err))
	}

	// wait for collector to write the service entry - retry for up to 3s
	var svc *Service
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		services, _ = loadServices()
		if s, ok := services[name]; ok {
			svc = s
			break
		}
	}
	if svc == nil {
		errBytes, _ := os.ReadFile(winkDir() + "/collector-err.log")
		if len(errBytes) > 0 {
			fatal(fmt.Errorf("collector error: %s", strings.TrimSpace(string(errBytes))))
		}
		fatal(fmt.Errorf("collector failed to start service %q\n  self=%s\n  cwd=%s\n  cmd=%s",
			name, self, cwd, strings.Join(cmdArgs, " ")))
	}

	fmt.Printf("  %s%s%s  %sstarted%s  pid %s%d%s\n", bold, name, reset, green, reset, dim, svc.PID, reset)
	fmt.Printf("  %slogs: wink logs %s  |  tail: wink tail %s%s\n", dim, name, name, reset)
}

// runCollector is the daemon: starts the actual process, streams its output to the log file
func runCollector(name string, cmdArgs []string) {
	_ = ensureDir()
	_ = os.WriteFile(winkDir()+"/collector-err.log",
		[]byte(fmt.Sprintf("collector started: %s %v\n", name, cmdArgs)), 0644)

	// clear previous logs for this service before writing new ones
	if lines, err := readLogs(""); err == nil {
		if f, err := os.Create(logsPath()); err == nil {
			enc := json.NewEncoder(f)
			for _, l := range lines {
				if l.Service != name {
					enc.Encode(l)
				}
			}
			f.Close()
		}
	}

	collectorFatal := func(msg string) {
		_ = os.WriteFile(winkDir()+"/collector-err.log", []byte(msg+"\n"), 0644)
		os.Exit(1)
	}

	shellCmd := strings.Join(cmdArgs, " ")
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Dir = func() string { d, _ := os.Getwd(); return d }()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		collectorFatal(fmt.Sprintf("stdout pipe failed: %v", err))
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		collectorFatal(fmt.Sprintf("stderr pipe failed: %v", err))
	}

	if err := cmd.Start(); err != nil {
		collectorFatal(fmt.Sprintf("cmd.Start failed: %v (cmd=%s, cwd=%s)", err, cmdArgs[0], func() string { d, _ := os.Getwd(); return d }()))
	}

	// register service with the actual process PID (locked to avoid race with other collectors)
	startedAt := time.Now()
	pid := cmd.Process.Pid
	cmdStr := strings.Join(cmdArgs, " ")
	_ = updateService(name, func(services map[string]*Service) {
		services[name] = &Service{
			Name:      name,
			Cmd:       cmdStr,
			PID:       pid,
			Status:    StatusRunning,
			StartedAt: startedAt,
		}
	})

	done := make(chan struct{}, 2)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			_ = appendLog(LogLine{
				Service:   name,
				Text:      scanner.Text(),
				Stream:    "stdout",
				Timestamp: time.Now(),
			})
		}
		done <- struct{}{}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			_ = appendLog(LogLine{
				Service:   name,
				Text:      scanner.Text(),
				Stream:    "stderr",
				Timestamp: time.Now(),
			})
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	err = cmd.Wait()
	stoppedAt := time.Now()
	finalStatus := StatusStopped
	if err != nil {
		finalStatus = StatusDead
	}
	_ = updateService(name, func(services map[string]*Service) {
		if svc, ok := services[name]; ok {
			svc.Status = finalStatus
			svc.StoppedAt = stoppedAt
		}
	})
}

func cmdRestart(name string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	svc, ok := services[name]
	if !ok {
		fatal(fmt.Errorf("service %q not found", name))
	}

	savedCmd := svc.Cmd

	// stop if running
	if svc.Status == StatusRunning {
		proc, err := os.FindProcess(svc.PID)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		// wait for it to stop
		for i := 0; i < 20; i++ {
			time.Sleep(200 * time.Millisecond)
			svcs, _ := loadServices()
			if s, ok := svcs[name]; ok && s.Status != StatusRunning {
				break
			}
		}
	}

	// clear old logs for this service
	lines, _ := readLogs("")
	f, err := os.Create(logsPath())
	if err == nil {
		enc := json.NewEncoder(f)
		for _, l := range lines {
			if l.Service != name {
				enc.Encode(l)
			}
		}
		f.Close()
	}

	fmt.Printf("  %srestarting%s  %s%s%s\n", dim, reset, bold, name, reset)
	cmdWatch(name, strings.Fields(savedCmd))
}

func cmdAttach(name string, pidStr string) {
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fatal(fmt.Errorf("invalid pid: %s", pidStr))
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fatal(fmt.Errorf("process %d not found", pid))
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		fatal(fmt.Errorf("process %d is not running", pid))
	}

	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	services[name] = &Service{
		Name:      name,
		Cmd:       fmt.Sprintf("(attached pid %d)", pid),
		PID:       pid,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}
	if err := saveServices(services); err != nil {
		fatal(err)
	}

	fmt.Printf("  %s%s%s  %sattached%s  pid %s%d%s\n", bold, name, reset, green, reset, dim, pid, reset)
	fmt.Printf("  %snote: wink cannot collect logs from attached processes%s\n", dim, reset)
}

func cmdStop(name string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	svc, ok := services[name]
	if !ok {
		fatal(fmt.Errorf("service %q not found", name))
	}
	if svc.Status != StatusRunning {
		fatal(fmt.Errorf("service %q is not running", name))
	}

	proc, err := os.FindProcess(svc.PID)
	if err != nil {
		fatal(fmt.Errorf("process %d not found", svc.PID))
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fatal(fmt.Errorf("failed to stop process: %w", err))
	}

	svc.Status = StatusStopped
	svc.StoppedAt = time.Now()
	if err := saveServices(services); err != nil {
		fatal(err)
	}

	fmt.Printf("  %s%s%s  %sstopped%s\n", bold, name, reset, yellow, reset)
}
