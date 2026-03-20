package main

import (
	"fmt"
	"os"
	"strings"
)

// ServiceDef holds a service definition parsed from wink.yaml.
type ServiceDef struct {
	Cmd     string
	Dir     string
	Restart bool
}

// parseWinkFile supports two formats:
//
// Simple (runs from wink up cwd):
//
//	api: node server.js
//
// Extended (explicit working dir):
//
//	api:
//	  cmd: node server.js
//	  dir: ./apps/api
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

func cmdUp(configPath string) {
	services, err := parseWinkFile(configPath)
	if err != nil {
		fatal(fmt.Errorf("reading %s: %w", configPath, err))
	}

	running, _ := loadServices()

	for _, name := range sortedKeys(toServiceMap(services)) {
		def := services[name]
		if svc, ok := running[name]; ok && svc.Status == StatusRunning {
			fmt.Printf("  %s%s%s  already running\n", dim, name, reset)
			continue
		}
		cmdWatch(name, strings.Fields(def.Cmd), def.Dir, def.Restart)
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

// toServiceMap converts ServiceDef map to Service map for sortedKeys compatibility.
func toServiceMap(m map[string]ServiceDef) map[string]*Service {
	out := make(map[string]*Service, len(m))
	for k := range m {
		out[k] = &Service{Name: k}
	}
	return out
}
