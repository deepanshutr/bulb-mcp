// Command bulb-mcp is the unified multi-protocol smart-bulb MCP stdio server.
// It exposes 13 tools, all delegating to bulb-cli/pkg/multiplex.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/deepanshutr/bulb-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// MCP uses stdout for the JSON-RPC wire; all logs MUST go to stderr.
	log.SetOutput(os.Stderr)
	if logPath := os.Getenv("BULB_MCP_LOG"); logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			log.SetOutput(f)
		}
	}

	m := multiplex.New(multiplex.LoadDaemonURLs())

	s := server.NewMCPServer("bulb-mcp", "0.1.0",
		server.WithToolCapabilities(false),
	)
	tools.Register(s, m)

	log.Printf("bulb-mcp starting; 13 tools registered")
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
