package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds user-configurable settings loaded from ~/.wink/config.yaml.
type Config struct {
	// NotifyCmd is a shell command run on service crash.
	// Use {msg} as placeholder for the notification message.
	// Example: terminal-notifier -title wink -message {msg}
	NotifyCmd string

	// MaxLogLines caps total lines kept in logs.jsonl.
	// Oldest lines are trimmed automatically. 0 means no limit.
	MaxLogLines int
}

func configPath() string {
	return filepath.Join(winkDir(), "config.yaml")
}

// defaultConfig is written to ~/.wink/config.yaml on first use.
const defaultConfig = `# wink config — ~/.wink/config.yaml
#
# notify_cmd: shell command to run when a service crashes.
#             use {msg} as placeholder for the notification message.
#             if not set, wink uses the platform default (osascript on macOS, notify-send on Linux).
#
# examples:
#   notify_cmd: terminal-notifier -title wink -message {msg} -sound Basso
#   notify_cmd: alerter -title wink -message {msg}
#   notify_cmd: notify-send wink {msg}
#   notify_cmd: ~/scripts/my-notify.sh {msg}

# notify_cmd:

# max_log_lines: maximum number of log lines to keep in logs.jsonl (0 = no limit).
#   oldest lines are trimmed automatically every 100 appends.
#   example: max_log_lines: 5000

# max_log_lines:
`

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseConfig(string(data)), nil
}

func parseConfig(raw string) *Config {
	cfg := &Config{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch key {
		case "notify_cmd":
			cfg.NotifyCmd = val
		case "max_log_lines":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.MaxLogLines = n
			}
		}
	}
	return cfg
}

func saveConfig(c *Config) error {
	_ = ensureDir()

	// read existing file or start from default template
	existing, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		existing = []byte(defaultConfig)
	} else if err != nil {
		return err
	}

	updated := setConfigKey(string(existing), "notify_cmd", c.NotifyCmd)
	maxLines := ""
	if c.MaxLogLines > 0 {
		maxLines = strconv.Itoa(c.MaxLogLines)
	}
	updated = setConfigKey(updated, "max_log_lines", maxLines)
	return os.WriteFile(configPath(), []byte(updated), 0644)
}

// setConfigKey sets or comments out a key in the raw config string.
func setConfigKey(raw, key, value string) string {
	lines := strings.Split(raw, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// match active or commented-out key
		active := strings.HasPrefix(trimmed, key+":")
		commented := strings.HasPrefix(trimmed, "# "+key+":") || strings.HasPrefix(trimmed, "#"+key+":")
		if active || commented {
			found = true
			if value == "" {
				lines[i] = fmt.Sprintf("# %s:", key)
			} else {
				lines[i] = fmt.Sprintf("%s: %s", key, value)
			}
			break
		}
	}
	if !found && value != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.Join(lines, "\n")
}

func ensureConfigFile() error {
	if _, err := os.Stat(configPath()); os.IsNotExist(err) {
		_ = ensureDir()
		return os.WriteFile(configPath(), []byte(defaultConfig), 0644)
	}
	return nil
}
