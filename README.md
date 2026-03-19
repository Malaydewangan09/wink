# wink

Log aggregator for local dev services. Watch any process, see all output in one place.

```
wink watch api "node server.js"
wink watch worker "python worker.py"
wink watch db "postgres -D /usr/local/var/postgres"
wink
```

Opens a live TUI: services on the left, aggregated logs on the right.

## Install

**macOS (Apple Silicon)**
```
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-darwin-arm64
chmod +x wink && sudo mv wink /usr/local/bin/
```

**macOS (Intel)**
```
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-darwin-x86_64
chmod +x wink && sudo mv wink /usr/local/bin/
```

**Linux (amd64)**
```
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-linux-amd64
chmod +x wink && sudo mv wink /usr/local/bin/
```

## Commands

```
wink watch <name> <cmd>    start a process and collect its output
wink attach <name> <pid>   register an already-running process
wink stop <name>           stop a watched service
wink ls                    list all services and status
wink logs [name]           show logs, optionally filtered by service
wink tail [name]           follow logs in real time
wink clear                 clear all logs and sessions
```

## TUI controls

```
tab      switch between services and logs pane
↑ ↓      navigate services or scroll logs
a        show all services logs
s        stop selected service
G        jump to bottom
q        quit
```

## How it works

`wink watch` spawns a background collector process that runs your command and streams stdout and stderr to `~/.wink/logs.jsonl`. The TUI reads this file every 500ms and renders the output. All state is stored in `~/.wink/`.

Services are machine-level, not project-scoped. Watch anything: Node, Python, Go, Java, shell scripts.

## Notes

- `wink attach` registers a running process but cannot collect its logs (pipe was not set up at start).
- Log files grow unbounded. Run `wink clear` after stopping all services to reset.

## License

MIT
