package tools

import (
	"context"
	"fmt"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/deepanshutr/bulb-cli/pkg/multiplex/groups"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerGroups attaches the two group-management tools (amendment A5.4),
// taking bulb-mcp to 13 tools. Control tools need no change — they accept
// home/zone:/room: as `target` already.
func registerGroups(s *server.MCPServer, _ *multiplex.Multiplexer) {
	s.AddTool(mcp.NewTool("bulb_group_list",
		mcp.WithDescription("Return the bulb group hierarchy: home -> zones -> rooms "+
			"-> member bulb MACs."),
	), wrap(func(_ context.Context, _ mcp.CallToolRequest) (string, error) {
		st, err := groups.Load()
		if err != nil {
			return "", err
		}
		return jsonString(st)
	}))

	s.AddTool(mcp.NewTool("bulb_group_assign",
		mcp.WithDescription("Assign or unassign bulbs to a room. action='assign' moves "+
			"the given MACs into <room> (removing them from any prior room); "+
			"action='unassign' removes them from whatever room they are in."),
		mcp.WithString("action", mcp.Required(),
			mcp.Description("'assign' or 'unassign'")),
		mcp.WithString("room",
			mcp.Description("Target room name (required when action='assign')")),
		mcp.WithArray("macs", mcp.Required(),
			mcp.Description("Bulb MACs to assign or unassign"),
			mcp.Items(map[string]any{"type": "string"})),
	), wrap(func(_ context.Context, req mcp.CallToolRequest) (string, error) {
		action, err := req.RequireString("action")
		if err != nil {
			return "", err
		}
		macs, err := stringSlice(req, "macs")
		if err != nil {
			return "", err
		}
		st, err := groups.Load()
		if err != nil {
			return "", err
		}
		switch action {
		case "assign":
			room := req.GetString("room", "")
			if room == "" {
				return "", fmt.Errorf("room is required when action='assign'")
			}
			if err := st.Assign(room, macs); err != nil {
				return "", err
			}
		case "unassign":
			if err := st.Unassign(macs); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("unknown action %q; want 'assign' or 'unassign'", action)
		}
		if err := st.Save(); err != nil {
			return "", err
		}
		return jsonString(st)
	}))
}

// stringSlice extracts a required string-array argument from a tool request.
func stringSlice(req mcp.CallToolRequest, name string) ([]string, error) {
	raw, ok := req.GetArguments()[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", name)
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("%s entries must be strings", name)
		}
		out = append(out, s)
	}
	return out, nil
}
