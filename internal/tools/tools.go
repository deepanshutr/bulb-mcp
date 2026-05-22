// Package tools registers the bulb MCP tools against an mcp-go server. Every
// tool delegates to bulb-cli/pkg/multiplex.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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
		mcp.WithDescription("Turn a bulb, group, or 'all' on."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		return opResult(m.Op(ctx, pick(req), multiplex.OpOn()))
	}))

	s.AddTool(mcp.NewTool("bulb_off",
		mcp.WithDescription("Turn a bulb, group, or 'all' off."),
		mcp.WithString("target", mcp.Description(targetDesc)),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		return opResult(m.Op(ctx, pick(req), multiplex.OpOff()))
	}))

	s.AddTool(mcp.NewTool("bulb_brightness",
		mcp.WithDescription("Set brightness, integer 10-100, on a bulb/group/'all'."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("level", mcp.Required(), mcp.Description("Brightness 10..100")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		level, err := req.RequireInt("level")
		if err != nil {
			return "", err
		}
		return opResult(m.Op(ctx, pick(req), multiplex.OpBrightness(level)))
	}))

	s.AddTool(mcp.NewTool("bulb_temp",
		mcp.WithDescription("Set color temperature in Kelvin, 2200-6500, on a bulb/group/'all'."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithNumber("kelvin", mcp.Required(), mcp.Description("Color temp 2200..6500 K")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		k, err := req.RequireInt("kelvin")
		if err != nil {
			return "", err
		}
		return opResult(m.Op(ctx, pick(req), multiplex.OpTemp(k)))
	}))

	s.AddTool(mcp.NewTool("bulb_color",
		mcp.WithDescription("Set RGB color (each 0-255) on a bulb/group/'all'."),
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
		return opResult(m.Op(ctx, pick(req), multiplex.OpColor(r, g, b)))
	}))

	s.AddTool(mcp.NewTool("bulb_scene",
		mcp.WithDescription("Activate a built-in scene by name or id on a bulb/group/'all'. "+
			"Scene sets differ per protocol; use bulb_list / the daemon /scenes catalog."),
		mcp.WithString("target", mcp.Description(targetDesc)),
		mcp.WithString("scene", mcp.Required(), mcp.Description("Scene name or id")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		scene, err := req.RequireString("scene")
		if err != nil {
			return "", err
		}
		return opResult(m.Op(ctx, pick(req), multiplex.OpScene(scene)))
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
