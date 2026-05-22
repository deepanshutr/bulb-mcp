package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deepanshutr/bulb-cli/pkg/multiplex"
)

// contextContext is context.Context; aliased so the SetScanFn call below reads
// without an inline import edit.
type contextContext = context.Context

func TestRegister_HasOnboardAndHealth(t *testing.T) {
	m := multiplex.New(multiplex.DaemonURLs{"wiz": "http://127.0.0.1:1"})
	names := registeredToolNames(t, m)
	if !names["bulb_onboard"] {
		t.Fatalf("bulb_onboard not registered; have %v", names)
	}
	if !names["bulb_health"] {
		t.Fatalf("bulb_health not registered; have %v", names)
	}
}

func TestTool_Health_ReportsDaemons(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer up.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": up.URL})
	res := callTool(t, m, "bulb_health", map[string]any{})
	if res.IsError {
		t.Fatalf("bulb_health error: %v", res.Content)
	}
	if !strings.Contains(textOf(res), "wiz") {
		t.Fatalf("bulb_health text: %s", textOf(res))
	}
}

func TestTool_Onboard_Aggregates(t *testing.T) {
	wiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"onboarded":[{"mac":"d8a011aaaa","name":"bulb-8"}]}`))
	}))
	defer wiz.Close()
	m := multiplex.New(multiplex.DaemonURLs{"wiz": wiz.URL})
	m.SetScanFn(func(_ contextContext) ([]string, error) {
		return []string{"wiz_d8a011aa"}, nil
	})
	res := callTool(t, m, "bulb_onboard", map[string]any{
		"ssid": "HomeWiFi", "password": "pw",
	})
	if res.IsError {
		t.Fatalf("bulb_onboard error: %v", res.Content)
	}
	if !strings.Contains(textOf(res), "bulb-8") {
		t.Fatalf("bulb_onboard text: %s", textOf(res))
	}
}
