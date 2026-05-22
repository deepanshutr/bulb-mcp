# CLAUDE.md — bulb-mcp

MCP stdio server for unified multi-protocol bulb control. 13 tools, all
delegating to `bulb-cli/pkg/multiplex`.

## Conventions

- Go 1.23, libraries: `github.com/mark3labs/mcp-go`, `github.com/deepanshutr/bulb-cli`
- `unset GOROOT; export GOPROXY=https://proxy.golang.org,direct` before any `go` command
- Per-repo: `git config user.email 52166434+deepanshutr@users.noreply.github.com`
- `log.SetOutput(os.Stderr)` is mandatory — MCP uses stdout; a stray stdout log
  line corrupts the wire format
- Run `go vet ./...`, `go test ./... -count=1`, `./scripts/smoke.sh` before commit

## Layout

```
cmd/bulb-mcp/main.go        MCP stdio server entrypoint
internal/tools/tools.go     9 atomic bulb_* tools
internal/tools/extras.go    bulb_onboard + bulb_health
internal/tools/groups.go    bulb_group_list + bulb_group_assign
scripts/smoke.sh            stdio JSON-RPC initialize + tools/list; asserts 13 tools
```

## No copied client

There is NO internal HTTP client. All dispatch logic lives in
`bulb-cli/pkg/multiplex`. To change behavior, change that package.

## Register with Claude Code

```bash
claude mcp add bulb ~/.local/bin/bulb-mcp -s user
```

Restart the running Claude session — MCP tool schemas are read at init.
