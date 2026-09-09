package preferences

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Payload size caps. View states carry a filter tree and sort rules, so they
// get generous headroom; scalar settings are tiny.
const (
	scalarMax    = 256
	viewStateMax = 64 * 1024
)

// ---- validators ----

// stringOneOf accepts a JSON string drawn from a fixed set.
func stringOneOf(allowed ...string) func(json.RawMessage) error {
	set := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return func(raw json.RawMessage) error {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("expected a string")
		}
		if _, ok := set[s]; !ok {
			return fmt.Errorf("value %q is not one of the allowed options", s)
		}
		return nil
	}
}

// intOneOf accepts a JSON integer drawn from a fixed set.
func intOneOf(allowed ...int) func(json.RawMessage) error {
	set := make(map[int]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return func(raw json.RawMessage) error {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			return fmt.Errorf("expected an integer")
		}
		if _, ok := set[n]; !ok {
			return fmt.Errorf("value %d is not one of the allowed options", n)
		}
		return nil
	}
}

// boolValue accepts any JSON boolean.
func boolValue(raw json.RawMessage) error {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return fmt.Errorf("expected a boolean")
	}
	return nil
}

// uuidOrEmpty accepts a JSON string that is either empty or a valid UUID. Empty
// means "no selection"; the frontend then picks an accessible default.
func uuidOrEmpty(raw json.RawMessage) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("expected a string")
	}
	if s == "" {
		return nil
	}
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("expected a UUID or empty string")
	}
	return nil
}

// jsonObject accepts any JSON object. It is intentionally permissive: view
// states are stored loosely and validated strictly at query time, where the
// dynamic filter runner treats every stored tree as untrusted input.
func jsonObject(raw json.RawMessage) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("expected a JSON object")
	}
	return nil
}

// ---- the registry ----
//
// Canonical keys and scopes mirror the agreed preference map. Every value here
// is also the source of truth for the frontend defaults: the browser never
// invents a durable default, it takes what the server returns.

func init() {
	// --- project-scoped view states ---
	register(Definition{
		Key: "tasks.list.view_state", Scope: ScopeProject, ValueVersion: 1, MaxBytes: viewStateMax,
		Default:  json.RawMessage(`{"filter":null,"sortBy":"created_at","sortDir":"desc","groupBy":"state","viewSlug":null}`),
		Validate: jsonObject,
	})
	register(Definition{
		Key: "board.view_state", Scope: ScopeProject, ValueVersion: 1, MaxBytes: viewStateMax,
		Default:  json.RawMessage(`{"filter":null,"sortBy":"created_at","sortDir":"desc","groupBy":"state","viewSlug":null}`),
		Validate: jsonObject,
	})
	register(Definition{
		Key: "modules.overview.view_state", Scope: ScopeProject, ValueVersion: 1, MaxBytes: viewStateMax,
		Default:  json.RawMessage(`{"sort_by":"created_at","sort_dir":"desc"}`),
		Validate: jsonObject,
	})
	// Note: cycle-detail and module-detail task lists are intentionally NOT
	// persisted. Their filter options (states, assignees) vary per cycle/module,
	// so a saved filter/sort/search would surface options that don't exist in
	// another instance. Those bars stay ephemeral (in-memory, reset per visit).

	// --- global view modes ---
	register(Definition{
		Key: "cycles.overview.view_mode", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`"card"`),
		Validate: stringOneOf("card", "list"),
	})
	register(Definition{
		Key: "modules.overview.view_mode", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`"card"`),
		Validate: stringOneOf("card", "list"),
	})

	// --- global detail-panel visibility (cycles and modules kept separate) ---
	register(Definition{
		Key: "cycles.detail.panel_visible", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`true`),
		Validate: boolValue,
	})
	register(Definition{
		Key: "modules.detail.panel_visible", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`true`),
		Validate: boolValue,
	})

	// --- other global scalars ---
	register(Definition{
		Key: "tasks.detail.activity_sort", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`"newest"`),
		Validate: stringOneOf("newest", "oldest"),
	})
	register(Definition{
		Key: "tasks.list.page_size", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`20`),
		Validate: intOneOf(20, 50, 100),
	})
	register(Definition{
		Key: "app.theme", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`"light"`),
		Validate: stringOneOf("light", "dark", "system"),
	})
	register(Definition{
		Key: "navigation.last_workspace_id", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		Default:  json.RawMessage(`""`),
		Validate: uuidOrEmpty,
	})
	register(Definition{
		Key: "dashboard.show_all_workspaces", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		// Defaults to true (show every workspace) to match the pre-existing
		// dashboard behavior before this preference was persisted.
		Default:  json.RawMessage(`true`),
		Validate: boolValue,
	})
	register(Definition{
		Key: "notifications.email_enabled", Scope: ScopeGlobal, ValueVersion: 1, MaxBytes: scalarMax,
		// Defaults to true: users receive email notifications unless they opt out
		// in Settings. In-app notifications are unaffected by this preference.
		Default:  json.RawMessage(`true`),
		Validate: boolValue,
	})
}
