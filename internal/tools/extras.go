package tools

import (
	"context"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerExtras attaches bulb_onboard and bulb_health (the two tools beyond
// the original 9 atomic ones, per spec §6 Phase B).
func registerExtras(s *server.MCPServer, m *multiplex.Multiplexer) {
	s.AddTool(mcp.NewTool("bulb_health",
		mcp.WithDescription("Report which protocol -core daemons (wiz/yeelight/tuya) "+
			"are currently reachable."),
	), wrap(func(ctx context.Context, _ mcp.CallToolRequest) (string, error) {
		res, err := m.Health(ctx)
		if err != nil {
			return "", err
		}
		return jsonString(res)
	}))

	s.AddTool(mcp.NewTool("bulb_onboard",
		mcp.WithDescription("Onboard setup-mode bulbs of every protocol onto a Wi-Fi "+
			"network. Scans for wiz_*, yeelink-*, and ESP_* setup SSIDs and POSTs "+
			"/onboard to each matching daemon."),
		mcp.WithString("ssid", mcp.Required(),
			mcp.Description("SSID of the home Wi-Fi the bulbs should join")),
		mcp.WithString("password", mcp.Required(),
			mcp.Description("Password for the home Wi-Fi")),
		mcp.WithNumber("timeout_s",
			mcp.Description("Per-protocol onboarding timeout in seconds (default 60)")),
		mcp.WithString("protocol",
			mcp.Description("Force every detected setup SSID to this protocol "+
				"(wiz|yeelight|tuya). Mutually exclusive with skip_protocol.")),
		mcp.WithString("skip_protocol",
			mcp.Description("Exclude this protocol from onboarding. "+
				"Mutually exclusive with protocol.")),
	), wrap(func(ctx context.Context, req mcp.CallToolRequest) (string, error) {
		ssid, err := req.RequireString("ssid")
		if err != nil {
			return "", err
		}
		password, err := req.RequireString("password")
		if err != nil {
			return "", err
		}
		timeout := req.GetInt("timeout_s", 60)
		res, err := m.Onboard(ctx, ssid, password, timeout, multiplex.OnboardOpts{
			ForceProtocol: req.GetString("protocol", ""),
			SkipProtocol:  req.GetString("skip_protocol", ""),
		})
		if err != nil {
			return "", err
		}
		return jsonString(res)
	}))
}
