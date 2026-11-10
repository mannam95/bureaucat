package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/swaggo/swag"
)

// hiddenDocPathMarkers hides routes from the served spec (and so from Scalar
// and anything else reading /api/v1/openapi.json) while their feature is still
// being tested. A path is hidden when it contains a marker; the matching
// definitions go with it. The routes themselves stay registered and guarded -
// this hides documentation, not access. Remove an entry to publish the docs.
var hiddenDocPathMarkers = []string{"/insights"}

// hiddenDocDefinitionPrefixes drops the schema definitions that only hidden
// paths reference.
var hiddenDocDefinitionPrefixes = []string{"insights."}

func hiddenDocPath(path string) bool {
	for _, m := range hiddenDocPathMarkers {
		if strings.Contains(path, m) {
			return true
		}
	}
	return false
}

// openAPISpec serves the swag-generated spec with hidden paths stripped and
// host and scheme rewritten to match the incoming request, so "try it" calls
// hit the current instance instead of the hardcoded localhost from the
// annotations.
func openAPISpec(c *echo.Context) error {
	doc, err := swag.ReadDoc()
	if err != nil {
		return echo.ErrInternalServerError
	}

	var spec map[string]any
	if err := json.Unmarshal([]byte(doc), &spec); err != nil {
		return echo.ErrInternalServerError
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
