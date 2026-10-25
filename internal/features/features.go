// Package features implements instance-wide feature gates. The super admin
// decides who can use a gated feature: a mode (disabled, admins, everyone)
// plus grants for specific people on top of the mode. Enforcement happens
// server-side on every gated route — hiding the UI is a courtesy, the
// middleware is the rule.
package features

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/auth"
	"bereaucat/internal/store"
)

// Gated feature keys. Like preference keys, features are a closed server-side
// registry: the admin API refuses keys that are not listed here, and stale
// gate rows for removed features are ignored.
const (
	Insights = "insights"
)

// registry lists every gated feature with its admin-facing description.
var registry = map[string]string{
	Insights: "Delivery reporting reconstructed from the activity log: the portfolio, project and person reports with their evidence drawers.",
}

// Known reports whether the feature key is registered.
func Known(feature string) bool {
	_, ok := registry[feature]
	return ok
}

// Describe returns the admin-facing description of a registered feature.
func Describe(feature string) string {
	return registry[feature]
}

// All returns the registered feature keys in stable order.
func All() []string {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Gate modes. Grants are additive on top of the mode, so "admins plus these
// two people" works; the super admin has no implicit bypass — access is only
// ever what the gate says.
const (
	ModeDisabled = "disabled"
	ModeAdmins   = "admins"
	ModeEveryone = "everyone"
)

// ValidMode reports whether m is a storable gate mode.
func ValidMode(m string) bool {
	return m == ModeDisabled || m == ModeAdmins || m == ModeEveryone
}

// Enabled reports whether the feature is usable by the given caller. A feature
// without a gate row is disabled, and grants cannot outlive their gate row, so
// the missing-row case is a plain no.
func Enabled(ctx context.Context, q store.Querier, feature string, userID uuid.UUID, userType string) (bool, error) {
	mode, err := q.GetFeatureGateMode(ctx, feature)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	switch mode {
	case ModeEveryone:
		return true, nil
	case ModeAdmins:
		if userType == "admin" {
			return true, nil
		}
	}
	return q.HasFeatureGrant(ctx, store.HasFeatureGrantParams{Feature: feature, UserID: userID})
}

// EnabledFor returns the registered features the caller can use — the list the
// UI reads off /me to decide which surfaces to show.
func EnabledFor(ctx context.Context, q store.Querier, userID uuid.UUID, userType string) ([]string, error) {
	gates, err := q.ListFeatureGates(ctx)
	if err != nil {
		return nil, err
	}
	granted, err := q.ListFeatureGrantsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, g := range gates {
		if !Known(g.Feature) {
			continue
		}
		ok := g.Mode == ModeEveryone ||
			(g.Mode == ModeAdmins && userType == "admin") ||
			slices.Contains(granted, g.Feature)
		if ok {
			out = append(out, g.Feature)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Require gates a route on a feature. It must run after auth.Middleware (it
// reads the user headers that middleware sets) and covers every credential
// kind, personal access tokens included.
func Require(q store.Querier, feature string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
			}
			userType := c.Request().Header.Get(auth.HeaderUserType)
			ok, err := Enabled(c.Request().Context(), q, feature, userID, userType)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to check feature access")
			}
			if !ok {
				return echo.NewHTTPError(http.StatusForbidden, "this feature is not enabled for your account")
			}
			return next(c)
		}
	}
}
