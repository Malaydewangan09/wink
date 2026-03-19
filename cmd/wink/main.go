package main

import (
	"fmt"
	"os"
)

const version = "0.2.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		cmdTUI()
		return
	}

	switch args[0] {
	case "__collect":
		// internal: run by watch as background daemon
		if len(args) < 3 {
			os.Exit(1)
		}
		runCollector(args[1], args[2:])
		return
	case "watch":
		if len(args) < 3 {
			fatal(fmt.Errorf("usage: wink watch <name> <cmd> [args...]"))
		}
		cmdWatch(args[1], args[2:])
	case "attach":
		if len(args) < 3 {
			fatal(fmt.Errorf("usage: wink attach <name> <pid>"))
		}
		cmdAttach(args[1], args[2])
	case "ls":
		cmdList()
	case "logs":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		cmdLogs(name)
	case "tail":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		cmdTail(name)
	case "stop":
		if len(args) < 2 {
			fatal(fmt.Errorf("usage: wink stop <name>"))
		}
		cmdStop(args[1])
	case "restart":
		if len(args) < 2 {
			fatal(fmt.Errorf("usage: wink restart <name>"))
		}
		cmdRestart(args[1])
	case "rm", "remove":
		if len(args) < 2 {
			fatal(fmt.Errorf("usage: wink rm <name>"))
		}
		cmdRemove(args[1])
	case "clear":
		cmdClear()
	case "version":
		fmt.Printf("wink %s\n", version)
	case "ui":
		cmdTUI()
	case "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%serror:%s %s\n", red, reset, err)
	os.Exit(1)
}

func printHelp() {
	fmt.Printf("\n  %s%swink%s  %slog aggregator for local services%s  %sv%s%s\n\n", bold, white, reset, dim, reset, dim, version, reset)
	fmt.Printf("  %sWATCH%s\n", bold, reset)
	fmt.Printf("  wink %swatch%s <name> <cmd>    start a process and collect its output\n", cyan, reset)
	fmt.Printf("  wink %sattach%s <name> <pid>   attach to an already-running process\n", cyan, reset)
	fmt.Printf("  wink %sstop%s <name>            stop watching a service\n", cyan, reset)
	fmt.Printf("  wink %srestart%s <name>         restart a service with the same command\n", cyan, reset)
	fmt.Printf("  wink %srm%s <name>              remove a service and its logs\n", cyan, reset)
	fmt.Printf("\n  %sVIEW%s\n", bold, reset)
	fmt.Printf("  wink %sls%s                     list all watched services\n", dim, reset)
	fmt.Printf("  wink %slogs%s [name]            show logs, optionally filter by service\n", dim, reset)
	fmt.Printf("  wink %stail%s [name]            follow logs in real time\n", dim, reset)
	fmt.Printf("  wink %sclear%s                  clear all logs and sessions\n", dim, reset)
	fmt.Printf("\n")
}
