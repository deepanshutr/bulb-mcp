# GitHub Secrets used by this repo

| Secret | Used in | Purpose |
|--------|---------|---------|
| `TG_BOT_TOKEN` | `release.yml` notify job | Post release notifications to operator's Telegram. Optional — notify is a no-op if unset. |
| `TG_CHAT_ID`   | `release.yml` notify job | Target chat. |

No runtime secrets. `bulb-mcp` is a local stdio MCP server; daemon URLs come
from `~/.config/bulb/daemons.json` or the `BULB_DAEMONS_JSON` / `WIZ_URL` /
`YEELIGHT_URL` / `TUYA_URL` env vars.
