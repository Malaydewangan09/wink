package main

import (
	"fmt"
	"os"
	"strings"
)

// parseWinkFile reads a wink.yaml file.
// Format: one service per line — name: command
// Lines starting with # are comments.
func parseWinkFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	services := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, cmd, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		cmd = strings.TrimSpace(cmd)
		if name != "" && cmd != "" {
			services[name] = cmd
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no services found in %s", path)
	}
	return services, nil
}

func cmdUp(configPath string) {
	services, err := parseWinkFile(configPath)
	if err != nil {
		fatal(fmt.Errorf("reading %s: %w", configPath, err))
	}

	running, _ := loadServices()

	for _, name := range sortedKeys(toServiceMap(services)) {
		cmd := services[name]
		if svc, ok := running[name]; ok && svc.Status == StatusRunning {
			fmt.Printf("  %s%s%s  already running\n", dim, name, reset)
			continue
		}
		cmdWatch(name, strings.Fields(cmd))
	}
}

func cmdDown(configPath string) {
	services, err := parseWinkFile(configPath)
	if err != nil {
		fatal(fmt.Errorf("reading %s: %w", configPath, err))
	}

	running, _ := loadServices()

	for _, name := range sortedKeys(toServiceMap(services)) {
		svc, ok := running[name]
		if !ok || svc.Status != StatusRunning {
			fmt.Printf("  %s%s%s  not running\n", dim, name, reset)
			continue
		}
		cmdStop(name)
	}
}

// toServiceMap converts name->cmd map to name->*Service map for sortedKeys compatibility.
func toServiceMap(m map[string]string) map[string]*Service {
	out := make(map[string]*Service, len(m))
	for k := range m {
		out[k] = &Service{Name: k}
	}
	return out
}
