package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
)

// service name colors - cycle through these
var labelColors = []string{cyan, green, magenta, yellow, blue}

func colorForService(name string, services map[string]*Service) string {
	keys := sortedKeys(services)
	for i, k := range keys {
		if k == name {
			return labelColors[i%len(labelColors)]
		}
	}
	return white
}

func sortedKeys(services map[string]*Service) []string {
	keys := make([]string, 0, len(services))
	for k := range services {
		keys = append(keys, k)
	}
	// simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func statusColor(s Status) string {
	switch s {
	case StatusRunning:
		return green
	case StatusStopped:
		return dim
	case StatusDead:
		return red
	}
	return dim
}

func statusDot(s Status) string {
	switch s {
	case StatusRunning:
		return "●"
	case StatusStopped:
		return "○"
	case StatusDead:
		return "✗"
	}
	return "○"
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 2 15:04")
	}
}

func cmdList() {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	if len(services) == 0 {
		fmt.Printf("\n  %sno services watched%s\n\n", dim, reset)
		return
	}

	fmt.Println()
	for _, name := range sortedKeys(services) {
		svc := services[name]
		color := colorForService(name, services)
		sc := statusColor(svc.Status)
		dot := statusDot(svc.Status)

		uptime := ""
		if svc.Status == StatusRunning {
			uptime = fmt.Sprintf("  %s%s%s", dim, relativeTime(svc.StartedAt), reset)
		}

		fmt.Printf("  %s%s%s  %s%s%s  %s%s%s  %s%s%s%s\n",
			sc, dot, reset,
			bold, color+name, reset,
			dim, svc.Cmd, reset,
			dim, string(svc.Status), reset,
			uptime)
	}
	fmt.Println()
}

func cmdLogs(filter string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	lines, err := readLogs(filter)
	if err != nil {
		fatal(err)
	}

	if len(lines) == 0 {
		if filter != "" {
			fmt.Printf("\n  %sno logs for %s%s\n\n", dim, filter, reset)
		} else {
			fmt.Printf("\n  %sno logs yet%s\n\n", dim, reset)
		}
		return
	}

	fmt.Println()
	for _, l := range lines {
		color := colorForService(l.Service, services)
		ts := l.Timestamp.Format("15:04:05")
		stream := ""
		if l.Stream == "stderr" {
			stream = fmt.Sprintf(" %s[err]%s", red, reset)
		}
		fmt.Printf("  %s%s%-10s%s  %s%s%s  %s%s%s%s\n",
			bold, color, l.Service, reset,
			dim, ts, reset,
			l.Text,
			stream, "", reset)
	}
	fmt.Println()
}

func cmdTail(filter string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	// print existing logs first
	lines, _ := readLogs(filter)
	// show last 20
	start := 0
	if len(lines) > 20 {
		start = len(lines) - 20
	}
	for _, l := range lines[start:] {
		color := colorForService(l.Service, services)
		ts := l.Timestamp.Format("15:04:05")
		stream := ""
		if l.Stream == "stderr" {
			stream = fmt.Sprintf(" %s[err]%s", red, reset)
		}
		fmt.Printf("  %s%s%-10s%s  %s%s%s  %s%s\n",
			bold, color, l.Service, reset,
			dim, ts, reset,
			l.Text, stream)
	}

	// tail new lines
	f, err := os.Open(logsPath())
	if err != nil {
		return
	}
	defer f.Close()

	// seek to end
	f.Seek(0, 2)

	fmt.Printf("\n  %sfollowing logs%s  ctrl+c to stop\n\n", dim, reset)

	scanner := bufio.NewScanner(f)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var l LogLine
			if err := json.Unmarshal([]byte(line), &l); err != nil {
				continue
			}
			if filter != "" && l.Service != filter {
				continue
			}
			// reload services for color (may change)
			services, _ = loadServices()
			color := colorForService(l.Service, services)
			ts := l.Timestamp.Format("15:04:05")
			stream := ""
			if l.Stream == "stderr" {
				stream = fmt.Sprintf(" %s[err]%s", red, reset)
			}
			fmt.Printf("  %s%s%-10s%s  %s%s%s  %s%s\n",
				bold, color, l.Service, reset,
				dim, ts, reset,
				l.Text, stream)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func cmdRemove(name string) {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	svc, ok := services[name]
	if !ok {
		fatal(fmt.Errorf("no service named %q", name))
	}

	if svc.Status == StatusRunning {
		fatal(fmt.Errorf("%s is still running. stop it first with wink stop %s", name, name))
	}

	// remove from services map
	delete(services, name)
	if err := saveServices(services); err != nil {
		fatal(err)
	}

	// rewrite logs file without this service's lines
	lines, _ := readLogs("")
	f, err := os.Create(logsPath())
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, l := range lines {
		if l.Service != name {
			enc.Encode(l)
		}
	}

	fmt.Printf("  %s%s%s  removed\n", dim, name, reset)
}

func cmdClear() {
	services, err := loadServices()
	if err != nil {
		fatal(err)
	}

	// stop running services first
	running := 0
	for _, svc := range services {
		if svc.Status == StatusRunning {
			running++
		}
	}
	if running > 0 {
		fmt.Printf("  %s%d service(s) still running. stop them first with wink stop <name>%s\n", yellow, running, reset)
		return
	}

	os.Remove(logsPath())
	os.Remove(servicesPath())
	fmt.Printf("  %scleared%s\n", dim, reset)
}

