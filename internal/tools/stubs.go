package tools

import (
	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/mark3labs/mcp-go/server"
)

// registerExtras is implemented in extras.go (Task 19). This stub is replaced
// there; it exists only so Task 18 compiles standalone.
func registerExtras(_ *server.MCPServer, _ *multiplex.Multiplexer) {}

// registerGroups is implemented in groups.go (Task 20). Stub for Task 18.
func registerGroups(_ *server.MCPServer, _ *multiplex.Multiplexer) {}
