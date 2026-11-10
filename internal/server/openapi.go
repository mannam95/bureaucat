package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/swaggo/swag"
)

// hiddenDocPathMarkers hides routes from the served spec (and so from Scalar
// and anything else reading /api/v1/openapi.json) while their feature is still
// being tested or is hidden on this fork. A path is hidden when it contains a
// marker; the matching definitions go with it. The routes themselves stay
// registered and guarded - this hides documentation, not access. Remove an
// entry to publish the docs.
var hiddenDocPathMarkers = []string{
	"/insights", // still being tested
	"/workflow", // the workflow engine is hidden on this fork
}

// hiddenDocDefinitionPrefixes drops the schema definitions that only hidden
// paths reference.
var hiddenDocDefinitionPrefixes = []string{
	"insights.",
	"handlers.Workflow",
	"handlers.SaveWorkflow",
	"handlers.SetWorkflowEnabled",
}

func hiddenDocPath(path string) bool {
	for _, m := range hiddenDocPathMarkers {
		if strings.Contains(path, m) {
			return true
		}
	}
	return false
}

// filteredSpec is the parsed spec with hidden paths stripped, built once: the
// spec is fixed at build time, and the route is public and unthrottled, so
// per-request parsing would be attacker-billable CPU.
var (
	specOnce     sync.Once
	filteredSpec map[string]any
	specErr      error
)

func loadFilteredSpec() (map[string]any, error) {
	specOnce.Do(func() {
		doc, err := swag.ReadDoc()
		if err != nil {
			specErr = err
			return
		}
		var spec map[string]any
		if err := json.Unmarshal([]byte(doc), &spec); err != nil {
			specErr = err
			return
		}
		if paths, ok := spec["paths"].(map[string]any); ok {
			for p := range paths {
				if hiddenDocPath(p) {
					delete(paths, p)
				}
			}
		}
		if defs, ok := spec["definitions"].(map[string]any); ok {
			for name := range defs {
				for _, prefix := range hiddenDocDefinitionPrefixes {
					if strings.HasPrefix(name, prefix) {
						delete(defs, name)
						break
					}
				}
			}
		}
		filteredSpec = spec
	})
	return filteredSpec, specErr
}

// openAPISpec serves the filtered spec with host and scheme rewritten to match
// the incoming request, so "try it" calls hit the current instance instead of
// the hardcoded localhost from the annotations.
func openAPISpec(c *echo.Context) error {
	base, err := loadFilteredSpec()
	if err != nil {
		return echo.ErrInternalServerError
	}
	// Shallow copy: only the top-level host/schemes keys differ per request,
	// and the nested maps are never mutated after the one-time filter.
	spec := make(map[string]any, len(base)+2)
	for k, v := range base {
		spec[k] = v
	}

	r := c.Request()
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	if host != "" {
		spec["host"] = host
		spec["schemes"] = []string{scheme}
	}

	return c.JSON(http.StatusOK, spec)
}
