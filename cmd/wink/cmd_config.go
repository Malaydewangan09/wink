package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func cmdConfigShow() {
	_ = ensureConfigFile()
	cfg, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	fmt.Printf("\n  %sconfig%s  %s\n\n", dim, reset, configPath())
	fmt.Printf("  %snotify_cmd%s     %s\n", cyan, reset,
		valueOrDim(cfg.NotifyCmd, "(not set — using platform default)"))
	maxLines := ""
	if cfg.MaxLogLines > 0 {
		maxLines = strconv.Itoa(cfg.MaxLogLines)
	}
	fmt.Printf("  %smax_log_lines%s  %s\n\n", cyan, reset,
		valueOrDim(maxLines, "(not set — no limit)"))
}

func cmdConfigEdit() {
	_ = ensureConfigFile()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, configPath())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(fmt.Errorf("editor exited: %w", err))
	}
}

func cmdConfigSet(key, value string) {
	_ = ensureConfigFile()
	cfg, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	switch key {
	case "notify_cmd":
		cfg.NotifyCmd = value
	case "max_log_lines":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			fatal(fmt.Errorf("max_log_lines must be a positive number"))
		}
		cfg.MaxLogLines = n
	default:
		fatal(fmt.Errorf("unknown config key %q", key))
	}
	if err := saveConfig(cfg); err != nil {
		fatal(err)
	}
	fmt.Printf("  %s%s%s  set\n", dim, key, reset)
}

func cmdConfigUnset(key string) {
	cfg, err := loadConfig()
	if err != nil {
		fatal(err)
	}
	switch key {
	case "notify_cmd":
		cfg.NotifyCmd = ""
	case "max_log_lines":
		cfg.MaxLogLines = 0
	default:
		fatal(fmt.Errorf("unknown config key %q", key))
	}
	if err := saveConfig(cfg); err != nil {
		fatal(err)
	}
	fmt.Printf("  %s%s%s  unset\n", dim, key, reset)
}

func valueOrDim(v, fallback string) string {
	if v == "" {
		return dim + fallback + reset
	}
	return v
}
