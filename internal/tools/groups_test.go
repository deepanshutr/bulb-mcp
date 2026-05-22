package tools

import (
	"strings"
	"testing"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
)

func TestRegister_ThirteenToolsTotal(t *testing.T) {
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	names := registeredToolNames(t, m)
	want := []string{
		"bulb_list", "bulb_state", "bulb_on", "bulb_off", "bulb_brightness",
		"bulb_temp", "bulb_color", "bulb_scene", "bulb_discover",
		"bulb_onboard", "bulb_health",
		"bulb_group_list", "bulb_group_assign",
	}
	for _, w := range want {
		if !names[w] {
			t.Fatalf("tool %q missing; have %v", w, names)
		}
	}
	if len(names) != 13 {
		t.Fatalf("expected exactly 13 tools, got %d: %v", len(names), names)
	}
}

func TestTool_GroupList_ReturnsTree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	// seed a zone so the tree is non-empty
	st := m.Groups()
	_ = st.AddZone("upstairs")
	_ = st.Save()
	res := callTool(t, m, "bulb_group_list", map[string]any{})
	if res.IsError {
		t.Fatalf("bulb_group_list error: %v", res.Content)
	}
	if !strings.Contains(textOf(res), "upstairs") {
		t.Fatalf("group_list text: %s", textOf(res))
	}
}

func TestTool_GroupAssign_AssignsToRoom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	st := m.Groups()
	_ = st.AddZone("z")
	_ = st.AddRoom("bedroom", "z")
	_ = st.Save()
	res := callTool(t, m, "bulb_group_assign", map[string]any{
		"action": "assign", "room": "bedroom", "macs": []any{"d8a0118dc5c3"},
	})
	if res.IsError {
		t.Fatalf("bulb_group_assign error: %v", res.Content)
	}
	// reload from disk to confirm the assignment persisted
	reload := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	out := callTool(t, reload, "bulb_group_list", map[string]any{})
	if !strings.Contains(textOf(out), "d8a0118dc5c3") {
		t.Fatalf("assignment not persisted: %s", textOf(out))
	}
}

func TestTool_GroupAssign_Unassign(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	st := m.Groups()
	_ = st.AddZone("z")
	_ = st.AddRoom("r", "z")
	_ = st.Assign("r", []string{"d8a0118dc5c3"})
	_ = st.Save()
	res := callTool(t, m, "bulb_group_assign", map[string]any{
		"action": "unassign", "macs": []any{"d8a0118dc5c3"},
	})
	if res.IsError {
		t.Fatalf("unassign error: %v", res.Content)
	}
}

func TestTool_GroupAssign_BadAction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	res := callTool(t, m, "bulb_group_assign", map[string]any{
		"action": "frobnicate", "macs": []any{"abc"},
	})
	if !res.IsError {
		t.Fatal("unknown action must be an error result")
	}
}
