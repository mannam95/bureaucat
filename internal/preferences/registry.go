// Package preferences defines the closed set of user-preference keys the API
// will accept, along with each key's scope, default value, and validation.
//
// The database table (user_preferences) is deliberately generic — any key and
// any JSON value would fit its columns — so this registry is what keeps the
// surface honest: the API rejects unknown keys, wrong-scope writes, oversized
// payloads, and malformed values. Defaults live here too, so a user with no
// stored row still gets the product default from the server rather than the
// browser guessing one.
package preferences

import (
	"encoding/json"
	"fmt"
)

// Scope says where a preference may be stored. A key is valid at exactly one
// scope; the API refuses to write it anywhere else.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	// ScopeWorkspace is reserved. The table has a workspace_id column and the
	// schema supports it, but no key uses it yet, so no workspace routes exist.
	ScopeWorkspace Scope = "workspace"
)

// Definition describes one known preference key.
type Definition struct {
	// Key is the canonical dotted identifier, e.g. "tasks.list.view_state".
	Key string
	// Scope is the only scope this key may be stored at.
	Scope Scope
	// ValueVersion is the current shape version of the stored JSON. It is
	// written alongside the value so a future release can migrate old shapes.
	ValueVersion int
	// MaxBytes caps the encoded JSON value; larger writes are rejected.
	MaxBytes int
	// Default is the effective value when the user has no stored row. It must
	// itself pass Validate.
	Default json.RawMessage
	// Validate rejects malformed or out-of-range values. It never sees a JSON
	// null (the table forbids one); callers guard that separately.
	Validate func(json.RawMessage) error
}

// registry is the closed set of accepted keys. Populated by register() from
// keys.go's init(). Nothing mutates it after startup.
var registry = map[string]Definition{}

func register(d Definition) {
	if _, dup := registry[d.Key]; dup {
		panic(fmt.Sprintf("preferences: duplicate key %q", d.Key))
	}
	if d.Validate == nil {
		panic(fmt.Sprintf("preferences: key %q has no validator", d.Key))
	}
	if err := d.Validate(d.Default); err != nil {
		panic(fmt.Sprintf("preferences: default for %q is invalid: %v", d.Key, err))
	}
	registry[d.Key] = d
}

// Lookup returns the definition for a key, or false if the key is unknown.
func Lookup(key string) (Definition, bool) {
	d, ok := registry[key]
	return d, ok
}

// ForScope returns every definition valid at the given scope, so the API can
// assemble the effective preference set (stored row, else default) for a scope.
func ForScope(scope Scope) []Definition {
	out := make([]Definition, 0, len(registry))
	for _, d := range registry {
		if d.Scope == scope {
			out = append(out, d)
		}
	}
	return out
}

// Check validates a candidate value for a key: known key, right scope, within
// size, and value-shaped correctly. Returned errors are safe to surface.
func (d Definition) Check(scope Scope, raw json.RawMessage) error {
	if d.Scope != scope {
		return fmt.Errorf("preference %q is not a %s-scoped setting", d.Key, scope)
	}
	if len(raw) == 0 {
		return fmt.Errorf("preference %q: value is required", d.Key)
	}
	if d.MaxBytes > 0 && len(raw) > d.MaxBytes {
		return fmt.Errorf("preference %q: value exceeds %d bytes", d.Key, d.MaxBytes)
	}
	// A bare JSON null can never be stored (DB check constraint) and is never a
	// meaningful preference; reject it before the validator runs.
	var probe any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("preference %q: invalid JSON: %w", d.Key, err)
	}
	if probe == nil {
		return fmt.Errorf("preference %q: value may not be null", d.Key)
	}
	return d.Validate(raw)
}
