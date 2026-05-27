package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	t.Setenv("BULB_STICKY_S", "0") // never fork a real bulb-sticky watcher from tests
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

// TestArmSticky_ForksWatcherWithExpectedArgs verifies the auto-armed sticky
// watcher is fork-exec'd with the right deadline + CLI args after a mutating
// tool runs. It points stickyBinPath at a tiny stub that records its argv to a
// file the test reads back.
func TestArmSticky_ForksWatcherWithExpectedArgs(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	stub := filepath.Join(dir, "sticky-stub.sh")
	const stubScript = "#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$ARGS_FILE\"\n"
	if err := os.WriteFile(stub, []byte(stubScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGS_FILE", argsFile)

	prevBin := stickyBinPath
	stickyBinPath = stub
	t.Cleanup(func() { stickyBinPath = prevBin })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bulbs" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"bulbs":[{"mac":"d8a0118dc5c3"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": srv.URL})

	res := callTool(t, m, "bulb_color", map[string]any{
		"target": "d8a0118dc5c3", "r": 255, "g": 0, "b": 0,
	})
	if res.IsError {
		t.Fatalf("bulb_color tool error: %v", res.Content)
	}

	// Poll briefly — the stub writes the file from its own subprocess.
	deadline := time.Now().Add(2 * time.Second)
	var got string
	for {
		if b, err := os.ReadFile(argsFile); err == nil && len(b) > 0 {
			got = string(b)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("sticky stub never wrote args to %s", argsFile)
		}
		time.Sleep(20 * time.Millisecond)
	}
	want := "900\ncolor\n255\n0\n0\nd8a0118dc5c3\n"
	if got != want {
		t.Fatalf("sticky args:\ngot:  %q\nwant: %q", got, want)
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
