# bulb-mcp

Unified multi-protocol smart-bulb MCP stdio server. Exposes 13 tools, all
delegating to [`bulb-cli/pkg/multiplex`](https://github.com/deepanshutr/bulb-cli)
which fronts the `wiz-core`, `yeelight-core`, and `tuya-core` daemons.

## Tools (13)

Atomic (9): `bulb_list`, `bulb_state`, `bulb_on`, `bulb_off`,
`bulb_brightness`, `bulb_temp`, `bulb_color`, `bulb_scene`, `bulb_discover`.

Extras (2): `bulb_onboard`, `bulb_health`.

Groups (2): `bulb_group_list`, `bulb_group_assign`.

Control tools accept `all`, `home`, `zone:<name>`, and `room:<name>` as the
`target`, so broadcast and group operations need no dedicated tools.

## Install

```bash
unset GOROOT; export GOPROXY=https://proxy.golang.org,direct
go build -o ~/.local/bin/bulb-mcp ./cmd/bulb-mcp
```

## Register with Claude Code

```bash
claude mcp add bulb ~/.local/bin/bulb-mcp -s user
```

Migrating from the old single-protocol server? `bulb migrate` (from `bulb-cli`)
runs `claude mcp remove philips-wiz-bulb && claude mcp add bulb … -s user` in
one shot. Restart any running Claude session afterward — MCP tool schemas are
read at session init.

## Configuration

Same as `bulb-cli`: `~/.config/bulb/daemons.json` or the `BULB_DAEMONS_JSON` /
`WIZ_URL` / `YEELIGHT_URL` / `TUYA_URL` env vars. `BULB_MCP_LOG` redirects logs
to a file (logs otherwise go to stderr — never stdout, which is the MCP wire).

## License

MIT.
