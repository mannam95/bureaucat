package server

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/swaggo/swag"
)

// openAPISpec serves the swag-generated spec with host and scheme rewritten to
// match the incoming request, so "try it" calls hit the current instance
// instead of the hardcoded localhost from the annotations.
func openAPISpec(c *echo.Context) error {
	doc, err := swag.ReadDoc()
	if err != nil {
		return echo.ErrInternalServerError
	}

	var spec map[string]any
	if err := json.Unmarshal([]byte(doc), &spec); err != nil {
		return echo.ErrInternalServerError
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
