package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// notifyCrash alerts when a service exits unexpectedly.
// Uses notify_cmd from config if set, otherwise falls back to platform defaults.
func notifyCrash(name string) {
	msg := fmt.Sprintf(notifyCrashFmt, name)

	cfg, _ := loadConfig()
	if cfg != nil && cfg.NotifyCmd != "" {
		cmd := strings.ReplaceAll(cfg.NotifyCmd, "{msg}", msg)
		_ = exec.Command("sh", "-c", cmd).Run()
		return
	}

	// no config — use platform default
	switch runtime.GOOS {
	case "darwin":
		// osascript is always available; works once Script Editor is allowed in Notifications
		script := fmt.Sprintf(`display notification %q with title %q sound name %q`, msg, appName, notifySound)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			// last resort: spoken alert, no permissions needed
			_ = exec.Command("say", fmt.Sprintf("%s: %s", appName, msg)).Run()
		}
	case "linux":
		_ = exec.Command("notify-send", "-u", "critical", appName, msg).Run()
	}
}
