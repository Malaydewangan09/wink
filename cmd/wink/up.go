package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ServiceDef holds a service definition parsed from wink.yaml.
type ServiceDef struct {
	Cmd       string
	Dir       string
	Restart   bool
	Port      int
	DependsOn string
}

// parseWinkFile supports two formats:
//
// Simple (runs from wink up cwd):
//
//	api: node server.js
//
// Extended (explicit working dir, port, deps):
//
//	api:
//	  cmd: node server.js
//	  dir: ./apps/api
//	  port: 3000
//	  depends_on: db
//	  restart: always
func parseWinkFile(path string) (map[string]ServiceDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	services := map[string]ServiceDef{}
	var currentBlock string

	for _, line := range strings.Split(string(data), "\n") {
		isIndented := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if isIndented && currentBlock != "" {
			// property of current block
			def := services[currentBlock]
			switch key {
			case "cmd":
				def.Cmd = val
			case "dir":
				def.Dir = val
			case "restart":
				def.Restart = val == "true" || val == "always"
			case "port":
				def.Port, _ = strconv.Atoi(val)
			case "depends_on":
				def.DependsOn = val
			}
			services[currentBlock] = def
		} else if val == "" {
			// block header: api:
			currentBlock = key
			services[key] = ServiceDef{}
		} else {
			// simple format: api: node server.js
			currentBlock = ""
			services[key] = ServiceDef{Cmd: val}
		}
	}

	// remove blocks with no cmd
	for name, def := range services {
		if def.Cmd == "" {
			delete(services, name)
		}
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services found in %s", path)
	}
	return services, nil
}

// topoSort returns service names in dependency order using DFS.
// Returns an error if a cycle or unknown dependency is found.
func topoSort(services map[string]ServiceDef) ([]string, error) {
	visited := map[string]bool{}
	inStack := map[string]bool{}
	var order []string

	var visit func(name string) error
	visit = func(name string) error {
		if inStack[name] {
			return fmt.Errorf("circular dependency at %q", name)
		}
		if visited[name] {
			return nil
		}
		inStack[name] = true
		dep := services[name].DependsOn
		if dep != "" {
			if _, ok := services[dep]; !ok {
				return fmt.Errorf("service %q depends on %q which is not defined", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		inStack[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for _, name := range sortedKeys(toServiceMap(services)) {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// waitForPort polls until the port accepts connections or 30s elapses.
func waitForPort(name string, port int) {
	addr := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("  %swaiting for %s on :%d%s\n", dim, name, port, reset)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("  %s:%d not ready after 30s, continuing%s\n", yellow, port, reset)
}

func cmdUp(configPath string) {
	services, err := parseWinkFile(configPath)
	if err != nil {
		fatal(fmt.Errorf("reading %s: %w", configPath, err))
	}

	order, err := topoSort(services)
	if err != nil {
		fatal(err)
	}

	running, _ := loadServices()

	for _, name := range order {
		def := services[name]
		if svc, ok := running[name]; ok && svc.Status == StatusRunning {
			fmt.Printf("  %s%s%s  already running\n", dim, name, reset)
			continue
		}
		cmdWatch(name, strings.Fields(def.Cmd), def.Dir, def.Restart, def.Port)
		// wait for port before starting dependents
		if def.Port > 0 {
			waitForPort(name, def.Port)
		}
	}
}

func cmdDown(configPath string) {
	services, err := parseWinkFile(configPath)
	if err != nil {
		fatal(fmt.Errorf("reading %s: %w", configPath, err))
	}

	running, _ := loadServices()

	// shut down in reverse dependency order
	order, err := topoSort(services)
	if err != nil {
		fatal(err)
	}
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		svc, ok := running[name]
		if !ok || svc.Status != StatusRunning {
			fmt.Printf("  %s%s%s  not running\n", dim, name, reset)
			continue
		}
		cmdStop(name)
	}
}

// toServiceMap converts ServiceDef map to Service map for sortedKeys compatibility.
func toServiceMap(m map[string]ServiceDef) map[string]*Service {
	out := make(map[string]*Service, len(m))
	for k := range m {
		out[k] = &Service{Name: k}
	}
	return out
}
