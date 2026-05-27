# THREAD_PICKUP — 2026-05-27 — bulb-mcp

## What was attempted

Operator request: "bake in a loop so that if I place any immediate
commands it runs till success for 15 mins at least … in general too
not just this session." Translation: every mutating bulb command
should keep retrying failed bulbs for 15 minutes, baked at the
MCP/handler layer so it works across Claude sessions without
re-explanation.

## What shipped

Three commits to main, in order:

1. **`6d3df80` — `chore: bump bulb-cli for resolveOwner-via-/bulbs
   fix`** — go.mod bumped to `bulb-cli v0.0.0-20260527150825-
   8473d7f961fa` (the perf fix). One test stub (`TestTool_On_Calls
   Multiplexer`) gained a `/bulbs` handler because the new
   resolveOwner hits it.

2. **`bab2275` — `feat(sticky): auto-arm bulb-sticky watcher after
   every mutating tool`** — every `bulb_{on,off,brightness,temp,
   color,scene}` MCP call now fork-execs `~/.local/bin/bulb-sticky`
   in the background (single-instance via PID file). The watcher
   retries every 30s for 15 min or until every bulb in the target
   set succeeds. `BULB_STICKY_S=0` disables; any positive int
   overrides the deadline.

3. **`0f27d1e` — `security(sticky): defuse argv flag smuggling on
   bulb-sticky fork-exec`** — fix from automated security review.
   Three defenses, in order:
   - `--` end-of-options sentinel between subcommand and user-
     influenced argv (defuses pflag flag-smuggling at the bulb CLI).
   - Allowlist regex `^[A-Za-z0-9_.:][A-Za-z0-9_.:-]*$` plus
     no-leading-`-` on user-influenced values. Signed integers
     accepted explicitly via `strconv.Atoi`.
   - `stickyBin` pinned to an absolute path via `exec.LookPath` at
     package init — `$PATH` substitution can't redirect the fork-
     exec to an attacker binary. Empty string disables (graceful
     degradation).
   - `armSticky` signature is now `(subcmd, subcmdArgs, target)`.

`~/.local/bin/bulb-mcp` was rebuilt after each commit. **Caveat:**
an already-running Claude session has the bulb-mcp from session
start; a session restart picks up the new code.

## Tests added

- `TestArmSticky_ForksWatcherWithExpectedArgs` — points
  `stickyBin` at a stub script in `t.TempDir`, asserts argv
  layout `900\ncolor\n--\n255\n0\n0\nd8a0118dc5c3\n`.
- `TestArmSticky_RejectsArgvFlagSmuggling` — scene `"-rf"` must
  NOT reach the stub.
- Existing `TestTool_On_CallsMultiplexer` gains `t.Setenv("BULB_
  STICKY_S", "0")` to prevent it spawning a real watcher.

## What's blocked

Nothing in this repo. The watcher script `~/.local/bin/bulb-
sticky` is deployed separately (bash, ~/.local/bin/, not in any
repo). If you change its argv contract, mirror it here and in
orchctl-v2.

## Resume incantation

```bash
cd ~/github.com/deepanshutr/bulb-mcp
unset GOROOT; export GOPROXY=https://proxy.golang.org,direct
git log --oneline -5      # 0f27d1e at HEAD
go test ./... -count=1
go build -o ~/.local/bin/bulb-mcp ./cmd/bulb-mcp
# verify Claude MCP picks up the new binary on next session:
claude mcp list           # bulb ✓ Connected
```

If editing armSticky here, mirror in `~/orchctl-v2/internal/bulb/
sticky.go` — same allowlist, same `--` placement.
