// Package tools registers the bulb MCP tools against an mcp-go server. Every
// tool delegates to bulb-cli/pkg/multiplex.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// stickyDeadlineDefault is how long the auto-armed bulb-sticky watcher keeps
// retrying after every mutating tool call. Override with BULB_STICKY_S=0 to
// disable, or any positive integer (seconds) to change. Default 900s = 15 min,
// per the operator's "bake in retry for at least 15 min" requirement.
const stickyDeadlineDefault = 900

// stickyBin is the resolved absolute path to bulb-sticky. Empty disables
// sticky arming (graceful degradation if the binary is not on PATH at
// process startup). Pinned at init so a later $PATH change cannot redirect
// the fork-exec to an attacker-controlled binary. Tests override directly.
var stickyBin = func() string {
	if p, err := exec.LookPath("bulb-sticky"); err == nil {
		return p
	}
	return ""
}()

// safeStickyArg matches user-influenced values that are safe to pass as
// positional argv to the bulb CLI. The character set is the union of all
// legitimate target/scene forms (MAC, IPv4, friendly names, group tokens
// like zone:upstairs); the first-char anchor forbids a leading '-' or '+',
// which is the only ASCII surface that pflag would treat as a flag prefix.
// Belt-and-braces alongside the explicit `--` sentinel placed by armSticky.
var safeStickyArg = regexp.MustCompile(`^[A-Za-z0-9_.:][A-Za-z0-9_.:-]*$`)

// targetDesc documents the shared `target` argument across control tools.
const targetDesc = "Bulb selector: a MAC (with or without colons), an IPv4, a " +
	"friendly name, 'all' (every discovered bulb), 'home' (every bulb assigned " +
	"to a room), 'zone:<name>', or 'room:<name>'. Omit to use the default bulb."

// Register attaches the 9 atomic bulb_* tools plus onboard, health, and the two
// group tools (registered in groups.go) to s.
func Register(s *server.MCPServer, m *multiplex.Multiplexer) {
	registerAtomic(s, m)
	registerExtras(s, m) // bulb_onboard + bulb_health (extras.go)
	registerGroups(s, m) // bulb_group_list + bulb_group_assign (groups.go)
}

// registerAtomic attaches the 9 atomic control/query tools.
func registerAtomic(s *server.MCPServer, m *multiplex.Multiplexer) {
	s.AddTool(mcp.NewTool("bulb_list",
		mcp.WithDescription("List every bulb across all protocol daemons (wiz/yeelight/tuya)."),
	), wrap(func(ctx context.Context, _ mcp.CallToolRequest) (string, error) {
		res, err := m.List(ctx)
		if err != nil {
			return "", err
		}
		return jsonString(res)
	}))

	s.AddTool(mcp.NewTool("bulb_state",
		mcp.WithDescription("Get a bulb's live state (on/off, brightness, color temp)."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		out, err := m.State(ctx, pick(req))
		if err != nil {
			return "", err
		}
		return jsonString(out)
	}))

	s.AddTool(mcp.NewTool("bulb_on",
		mcp.WithDescription("Turn a bulb, group, or 'all' on. Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		target := pick(req)
		out, err := opResult(m.Op(ctx, target, multiplex.OpOn()))
		armSticky("on", nil, target)
		return out, err
	}))

	s.AddTool(mcp.NewTool("bulb_off",
		mcp.WithDescription("Turn a bulb, group, or 'all' off. Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		target := pick(req)
		out, err := opResult(m.Op(ctx, target, multiplex.OpOff()))
		armSticky("off", nil, target)
		return out, err
	}))

	s.AddTool(mcp.NewTool("bulb_brightness",
		mcp.WithDescription("Set brightness, integer 10-100, on a bulb/group/'all'. Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("level", mcp.Required(), mcp.Description("Brightness 10..100")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		level, err := req.RequireInt("level")
		if err != nil {
			return "", err
		}
		target := pick(req)
		out, opErr := opResult(m.Op(ctx, target, multiplex.OpBrightness(level)))
		armSticky("bri", []string{strconv.Itoa(level)}, target)
		return out, opErr
	}))

	s.AddTool(mcp.NewTool("bulb_temp",
		mcp.WithDescription("Set color temperature in Kelvin, 2200-6500, on a bulb/group/'all'. Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("kelvin", mcp.Required(), mcp.Description("Color temp 2200..6500 K")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		k, err := req.RequireInt("kelvin")
		if err != nil {
			return "", err
		}
		target := pick(req)
		out, opErr := opResult(m.Op(ctx, target, multiplex.OpTemp(k)))
		armSticky("temp", []string{strconv.Itoa(k)}, target)
		return out, opErr
	}))

	s.AddTool(mcp.NewTool("bulb_color",
		mcp.WithDescription("Set RGB color (each 0-255) on a bulb/group/'all'. Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("r", mcp.Required(), mcp.Description("Red 0..255")),
		mcp.WithNumber("g", mcp.Required(), mcp.Description("Green 0..255")),
		mcp.WithNumber("b", mcp.Required(), mcp.Description("Blue 0..255")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		r, err := req.RequireInt("r")
		if err != nil {
			return "", err
		}
		g, err := req.RequireInt("g")
		if err != nil {
			return "", err
		}
		b, err := req.RequireInt("b")
		if err != nil {
			return "", err
		}
		target := pick(req)
		out, opErr := opResult(m.Op(ctx, target, multiplex.OpColor(r, g, b)))
		armSticky("color", []string{strconv.Itoa(r), strconv.Itoa(g), strconv.Itoa(b)}, target)
		return out, opErr
	}))

	s.AddTool(mcp.NewTool("bulb_scene",
		mcp.WithDescription("Activate a built-in scene by name or id on a bulb/group/'all'. "+
			"Scene sets differ per protocol; use bulb_list / the daemon /scenes catalog. "+
			"Arms a sticky watcher to retry failed bulbs."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithString("scene", mcp.Required(), mcp.Description("Scene name or id")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		scene, err := req.RequireString("scene")
		if err != nil {
			return "", err
		}
		target := pick(req)
		out, opErr := opResult(m.Op(ctx, target, multiplex.OpScene(scene)))
		armSticky("scene", []string{scene}, target)
		return out, opErr
	}))

	s.AddTool(mcp.NewTool("bulb_discover",
		mcp.WithDescription("Force a LAN rescan for new bulbs on every protocol daemon."),
	), wrap(func(ctx context.Context, _ mcp.CallToolRequest) (string, error) {
		res, err := m.Discover(ctx)
		if err != nil {
			return "", err
		}
		return jsonString(res)
	}))
}

// pick returns the request's `target` arg, defaulting to "_default".
func pick(req mcp.CallToolRequest) string {
	t := req.GetString("target", "")
	if t == "" {
		return "_default"
	}
	return t
}

// stickyDeadline reads BULB_STICKY_S; 0 disables; unset/invalid → default 900s.
func stickyDeadline() int {
	v := os.Getenv("BULB_STICKY_S")
	if v == "" {
		return stickyDeadlineDefault
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return stickyDeadlineDefault
	}
	return n
}

// armSticky fork-execs the bulb-sticky watcher detached so it survives the
// MCP request. The watcher script is single-instance via PID file —
// relaunching it replaces any prior sticky state automatically.
//
// Argv layout: <deadline_s> <subcmd> -- <subcmdArgs...> [target]
//
// `subcmd` is a literal from registerAtomic's switch (on/off/bri/temp/color/
// scene) and is trusted. Everything after the `--` is user-influenced — the
// sentinel tells the downstream bulb CLI's pflag parser to stop interpreting
// `-`-prefixed tokens as flags, defusing argv flag-smuggling. As a second
// layer, every user-influenced argv element is validated against
// safeStickyArg (with signed integers explicitly allowed); if any fails,
// arming is silently skipped — the immediate op already ran successfully
// and is the source of truth.
//
// Failures (binary missing, exec error) are logged and swallowed.
//
// target == "_default" means "no explicit target, use the daemon default"
// — we omit it from the CLI args so the bulb CLI also picks its default.
func armSticky(subcmd string, subcmdArgs []string, target string) {
	if stickyDeadline() == 0 || stickyBin == "" {
		return
	}
	for _, a := range subcmdArgs {
		if !isSafeStickyArg(a) {
			log.Printf("[bulb-mcp] sticky arm skipped (unsafe arg %q)", a)
			return
		}
	}
	if target != "" && target != "_default" && !isSafeStickyArg(target) {
		log.Printf("[bulb-mcp] sticky arm skipped (unsafe target %q)", target)
		return
	}
	args := []string{strconv.Itoa(stickyDeadline()), subcmd, "--"}
	args = append(args, subcmdArgs...)
	if target != "" && target != "_default" {
		args = append(args, target)
	}
	cmd := exec.Command(stickyBin, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		log.Printf("[bulb-mcp] sticky arm failed (%v); immediate op stands", err)
		return
	}
	// Reap the watcher PID asynchronously so it doesn't become a zombie.
	go func() { _ = cmd.Wait() }()
}

// isSafeStickyArg reports whether s is safe to forward as positional argv.
// Signed integers are accepted explicitly (the `--` sentinel insulates them
// from pflag); other values must match safeStickyArg.
func isSafeStickyArg(s string) bool {
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	return safeStickyArg.MatchString(s) && !strings.HasPrefix(s, "-")
}

// jsonString marshals v to an indented JSON string.
func jsonString(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// opResult renders a BroadcastResult as a JSON string, propagating an error.
func opResult(res multiplex.BroadcastResult, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if res.Total > 0 && res.OK == 0 {
		return "", fmt.Errorf("every bulb operation failed: %s", res.Op)
	}
	return jsonString(res)
}

// wrap adapts a (string, error) handler into an mcp-go tool handler. An error
// becomes a tool-result error rather than a transport error.
func wrap(fn func(context.Context, mcp.CallToolRequest) (string, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text, err := fn(ctx, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(text), nil
	}
}
