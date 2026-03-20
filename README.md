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
```bash
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-darwin-arm64
chmod +x wink && sudo mv wink /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-darwin-x86_64
chmod +x wink && sudo mv wink /usr/local/bin/
```

**Linux (amd64)**
```bash
curl -Lo wink https://github.com/Malaydewangan09/wink/releases/latest/download/wink-linux-amd64
chmod +x wink && sudo mv wink /usr/local/bin/
```

**From source**
```bash
go install github.com/Malaydewangan09/wink/cmd/wink@latest
```

## wink.yaml

Define services in a config file and start them all at once.

```yaml
# wink.yaml
api: go run ./cmd/api
worker: ./bin/worker --env dev
postgres: postgres -D /usr/local/var/postgresql
```

```bash
wink up      # start all services
wink down    # stop all services
```

## Commands

```
wink up [wink.yaml]        start all services from config file
wink down [wink.yaml]      stop all services from config file
wink watch <name> <cmd>    start a process and collect its output
wink attach <name> <pid>   register an already-running process
wink stop <name>           stop a watched service
wink restart <name>        restart with the same command
wink rm <name>             remove service and its logs
wink ls                    list all services and status
wink logs [name]           show logs, optionally filtered by service
wink tail [name]           follow logs in real time
wink clear                 clear all logs and sessions
wink config show           show current config
wink config edit           open config in $EDITOR
wink config set <key> <v>  set a config value
```

## TUI controls

```
↑ ↓      navigate services or scroll logs
tab      switch between services and logs pane
/        search logs by keyword (esc to clear)
t        toggle timestamps
a        show all services logs
s        stop selected service
r        restart selected service
x        remove selected service (press twice to confirm)
g / G    jump to top / bottom
q        quit
```

## Config

Settings are stored in `~/.wink/config.yaml`, created on first use.

```bash
wink config edit   # open in $EDITOR
```

| key | default | description |
|-----|---------|-------------|
| `notify_cmd` | platform default | shell command for crash notifications. use `{msg}` as placeholder |
| `max_log_lines` | 0 (no limit) | cap total lines in logs.jsonl, oldest trimmed automatically |

**Crash notifications** fire when a service exits with a non-zero code. Default is `osascript` on macOS, `notify-send` on Linux. Override with `notify_cmd`:

```bash
wink config set notify_cmd "terminal-notifier -title wink -message {msg}"
```

## How it works

`wink watch` spawns a background collector (`wink __collect`) as a detached process. The collector pipes stdout and stderr into `~/.wink/logs.jsonl` as newline-delimited JSON. The TUI reads that file on a 500ms tick. File locking (`flock`) prevents race conditions when multiple collectors write simultaneously.

State is stored in `~/.wink/`:
- `services.json` — service registry and status
- `logs.jsonl` — all log lines
- `config.yaml` — user settings

## Notes

- `wink attach` registers a running process but cannot collect its logs.
- `max_log_lines` trims the oldest lines across all services when the limit is hit.
- Processes that exit via SIGTERM/SIGINT are marked stopped, not dead. Only unexpected crashes trigger notifications.

## License

MIT
