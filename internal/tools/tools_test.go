package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registeredToolNames spins up a server, registers tools, and returns the set
// of tool names via an in-process tools/list call.
func registeredToolNames(t *testing.T, m *multiplex.Multiplexer) map[string]bool {
	t.Helper()
	s := server.NewMCPServer("bulb-mcp-test", "0.0.0", server.WithToolCapabilities(false))
	Register(s, m)
	names := map[string]bool{}
	for _, tl := range listTools(t, s) {
		names[tl] = true
	}
	return names
}

func TestRegister_NineAtomicTools(t *testing.T) {
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	names := registeredToolNames(t, m)
	for _, want := range []string{
		"bulb_list", "bulb_state", "bulb_on", "bulb_off", "bulb_brightness",
		"bulb_temp", "bulb_color", "bulb_scene", "bulb_discover",
	} {
		if !names[want] {
			t.Fatalf("atomic tool %q not registered; have %v", want, names)
		}
	}
}

func TestTool_On_CallsMultiplexer(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bulbs" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"bulbs":[{"mac":"d8a0118dc5c3"}]}`))
			return
		}
		path = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": srv.URL})
	res := callTool(t, m, "bulb_on", map[string]any{"target": "d8a0118dc5c3"})
	if res.IsError {
		t.Fatalf("bulb_on returned error: %v", res.Content)
	}
	if path != "/bulb/d8a0118dc5c3/on" {
		t.Fatalf("on path: %s", path)
	}
}

func TestTool_Temp_RequiresKelvin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": srv.URL})
	// missing kelvin -> tool result error, not a panic
	res := callTool(t, m, "bulb_temp", map[string]any{"target": "abc"})
	if !res.IsError {
		t.Fatal("bulb_temp without kelvin should be an error result")
	}
}

func TestTool_List_ReturnsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"bulbs":[{"mac":"w1","name":"bedroom","ip":"1"}]}`))
	}))
	defer srv.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": srv.URL})
	res := callTool(t, m, "bulb_list", map[string]any{})
	if res.IsError {
		t.Fatalf("bulb_list error: %v", res.Content)
	}
	if !strings.Contains(textOf(res), "bedroom") {
		t.Fatalf("bulb_list text: %s", textOf(res))
	}
}

// --- test helpers ---------------------------------------------------------

// listTools returns every registered tool's name.
func listTools(t *testing.T, s *server.MCPServer) []string {
	t.Helper()
	raw := s.HandleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]any{},
	}))
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal tools/list response: %v", err)
	}
	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	var names []string
	for _, tl := range resp.Result.Tools {
		names = append(names, tl.Name)
	}
	return names
}

// wireToolResult is a local struct that mirrors the JSON wire format of
// mcp.CallToolResult. The Content field is an interface slice which the
// standard json decoder cannot unmarshal directly, so we decode via the wire.
type wireToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

func (w *wireToolResult) toCallToolResult() *mcp.CallToolResult {
	ctr := &mcp.CallToolResult{IsError: w.IsError}
	for _, c := range w.Content {
		if c.Type == "text" {
			ctr.Content = append(ctr.Content, mcp.TextContent{Type: "text", Text: c.Text})
		}
	}
	return ctr
}

// callTool invokes one tool by name with the given args.
func callTool(t *testing.T, m *multiplex.Multiplexer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	s := server.NewMCPServer("bulb-mcp-test", "0.0.0", server.WithToolCapabilities(false))
	Register(s, m)
	raw := s.HandleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}))
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal tools/call response: %v", err)
	}
	var outer struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(b, &outer); err != nil {
		t.Fatalf("unmarshal tools/call outer: %v", err)
	}
	var wtr wireToolResult
	if err := json.Unmarshal(outer.Result, &wtr); err != nil {
		t.Fatalf("unmarshal wireToolResult: %v\nraw: %s", err, string(outer.Result))
	}
	return wtr.toCallToolResult()
}

// textOf concatenates the text content of a tool result.
func textOf(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// mustJSON marshals v to a json.RawMessage, failing the test on error.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
