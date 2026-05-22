#!/usr/bin/env bash
# Smoke test: drive the MCP stdio server with initialize + tools/list and
# assert exactly 13 tools are advertised (amendment A5.7).
set -euo pipefail
BIN="${1:-./bulb-mcp}"

OUT="$(
  {
    printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
    printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
    printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
  } | "$BIN"
)"

echo "$OUT" | head -3

COUNT="$(printf '%s' "$OUT" | grep -o '"name":"bulb_[a-z_]*"' | sort -u | wc -l | tr -d ' ')"
echo "tools advertised: $COUNT"
if [ "$COUNT" -ne 13 ]; then
  echo "FAIL: expected 13 tools, got $COUNT" >&2
  printf '%s' "$OUT" | grep -o '"name":"bulb_[a-z_]*"' | sort -u >&2
  exit 1
fi

for t in bulb_list bulb_state bulb_on bulb_off bulb_brightness bulb_temp \
         bulb_color bulb_scene bulb_discover bulb_onboard bulb_health \
         bulb_group_list bulb_group_assign; do
  if ! printf '%s' "$OUT" | grep -q "\"name\":\"$t\""; then
    echo "FAIL: tool $t not advertised" >&2
    exit 1
  fi
done
echo "OK: all 13 bulb tools present"
