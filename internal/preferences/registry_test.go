package preferences

import (
	"encoding/json"
	"strings"
	"testing"
)

// The registry is a closed set; these counts guard against a key being added or
// removed without a deliberate test update.
func TestForScopeCounts(t *testing.T) {
	if got := len(ForScope(ScopeGlobal)); got != 9 {
		t.Fatalf("global scope has %d keys, want 9", got)
	}
	if got := len(ForScope(ScopeProject)); got != 3 {
		t.Fatalf("project scope has %d keys, want 3", got)
	}
	if got := len(ForScope(ScopeWorkspace)); got != 0 {
		t.Fatalf("workspace scope has %d keys, want 0", got)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		key   string
		scope Scope
	}{
		{"app.theme", ScopeGlobal},
		{"tasks.list.page_size", ScopeGlobal},
		{"cycles.detail.panel_visible", ScopeGlobal},
		{"tasks.list.view_state", ScopeProject},
		{"board.view_state", ScopeProject},
		{"modules.overview.view_state", ScopeProject},
	}
	for _, c := range cases {
		def, ok := Lookup(c.key)
		if !ok {
			t.Fatalf("Lookup(%q) not found", c.key)
		}
		if def.Scope != c.scope {
			t.Fatalf("Lookup(%q) scope = %q, want %q", c.key, def.Scope, c.scope)
		}
	}

	if _, ok := Lookup("cycles.detail.tasks.view_state"); ok {
		t.Fatal("cycles.detail.tasks.view_state should not be registered (detail lists are not persisted)")
	}
	if _, ok := Lookup("made.up.key"); ok {
		t.Fatal("unknown key should not be found")
	}
}

// Every registered default must itself be a valid value for its scope. register()
// enforces this at init, but this keeps it covered explicitly.
func TestDefaultsAreValid(t *testing.T) {
	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		for _, def := range ForScope(scope) {
			if err := def.Check(scope, def.Default); err != nil {
				t.Fatalf("default for %q is invalid: %v", def.Key, err)
			}
		}
	}
}

func TestCheckRejectsWrongScope(t *testing.T) {
	theme, _ := Lookup("app.theme") // global
	if err := theme.Check(ScopeProject, json.RawMessage(`"dark"`)); err == nil {
		t.Fatal("global key accepted a project-scoped write")
	}

	tasks, _ := Lookup("tasks.list.view_state") // project
	if err := tasks.Check(ScopeGlobal, json.RawMessage(`{}`)); err == nil {
		t.Fatal("project key accepted a global-scoped write")
	}
}

func TestCheckValidValues(t *testing.T) {
	cases := []struct {
		key   string
		scope Scope
		value string
	}{
		{"app.theme", ScopeGlobal, `"dark"`},
		{"app.theme", ScopeGlobal, `"system"`},
		{"cycles.overview.view_mode", ScopeGlobal, `"list"`},
		{"cycles.detail.panel_visible", ScopeGlobal, `false`},
		{"tasks.detail.activity_sort", ScopeGlobal, `"oldest"`},
		{"tasks.list.page_size", ScopeGlobal, `50`},
		{"dashboard.show_all_workspaces", ScopeGlobal, `true`},
		{"navigation.last_workspace_id", ScopeGlobal, `""`},
		{"navigation.last_workspace_id", ScopeGlobal, `"3f1a1d2e-0000-4000-8000-000000000000"`},
		{"tasks.list.view_state", ScopeProject, `{"filter":null,"sortBy":"created_at","sortDir":"desc"}`},
		{"modules.overview.view_state", ScopeProject, `{"sort_by":"title","sort_dir":"asc"}`},
	}
	for _, c := range cases {
		def, ok := Lookup(c.key)
		if !ok {
			t.Fatalf("Lookup(%q) not found", c.key)
		}
		if err := def.Check(c.scope, json.RawMessage(c.value)); err != nil {
			t.Fatalf("Check(%q, %s) = %v, want nil", c.key, c.value, err)
		}
	}
}

func TestCheckInvalidValues(t *testing.T) {
	cases := []struct {
		key   string
		scope Scope
		value string
	}{
		{"app.theme", ScopeGlobal, `"grid"`},                   // not an allowed option
		{"cycles.overview.view_mode", ScopeGlobal, `"x"`},      // not card/list
		{"cycles.detail.panel_visible", ScopeGlobal, `"true"`}, // string, not bool
		{"tasks.list.page_size", ScopeGlobal, `7`},             // not 20/50/100
		{"tasks.list.page_size", ScopeGlobal, `"50"`},          // string, not int
		{"navigation.last_workspace_id", ScopeGlobal, `"not-a-uuid"`},
		{"dashboard.show_all_workspaces", ScopeGlobal, `1`}, // number, not bool
		{"tasks.list.view_state", ScopeProject, `[]`},       // array, not object
		{"tasks.list.view_state", ScopeProject, `"x"`},      // string, not object
	}
	for _, c := range cases {
		def, ok := Lookup(c.key)
		if !ok {
			t.Fatalf("Lookup(%q) not found", c.key)
		}
		if err := def.Check(c.scope, json.RawMessage(c.value)); err == nil {
			t.Fatalf("Check(%q, %s) = nil, want an error", c.key, c.value)
		}
	}
}

func TestCheckRejectsNullAndEmpty(t *testing.T) {
	theme, _ := Lookup("app.theme")
	if err := theme.Check(ScopeGlobal, json.RawMessage(`null`)); err == nil {
		t.Fatal("a JSON null value was accepted")
	}
	if err := theme.Check(ScopeGlobal, nil); err == nil {
		t.Fatal("an empty value was accepted")
	}
}

func TestCheckEnforcesMaxBytes(t *testing.T) {
	def, _ := Lookup("tasks.list.view_state") // 64 KiB cap
	// A structurally valid object that comfortably exceeds the cap.
	big := `{"filter":null,"note":"` + strings.Repeat("a", 70*1024) + `"}`
	if err := def.Check(ScopeProject, json.RawMessage(big)); err == nil {
		t.Fatal("an oversized value was accepted")
	}
}
