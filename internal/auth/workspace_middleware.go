package auth

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/store"
)

const (
	// HeaderWorkspaceID is the header name for the workspace ID.
	HeaderWorkspaceID = "X-Workspace-ID"
	// HeaderWorkspaceKey is the header name for the workspace key.
	HeaderWorkspaceKey = "X-Workspace-Key"
)

// WorkspaceMiddleware returns an Echo middleware that validates workspace access.
// It expects a :workspaceKey path parameter and the auth headers to be set.
// Global admins can access any workspace; other users must be a workspace member.
// Sets X-Workspace-ID and X-Workspace-Key headers for downstream handlers.
//
// Workspace membership carries no roles — it only governs visibility. Mutating
// workspace endpoints are gated separately by AdminMiddleware.
func WorkspaceMiddleware(queryer store.Querier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			workspaceKey := c.Param("workspaceKey")
			if workspaceKey == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "workspace key is required")
			}

			userIDStr := c.Request().Header.Get(HeaderUserID)
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
			}

			ctx := c.Request().Context()

			workspace, err := queryer.GetWorkspaceByKey(ctx, workspaceKey)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "workspace not found")
			}

			// Global admins can access all workspaces; others must be a member.
			if c.Request().Header.Get(HeaderUserType) != "admin" {
				isMember, err := queryer.IsWorkspaceMember(ctx, store.IsWorkspaceMemberParams{
					WorkspaceID: workspace.ID,
					UserID:      userID,
				})
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
				}
				if !isMember {
					return echo.NewHTTPError(http.StatusForbidden, "access denied: not a workspace member")
				}
			}

			c.Request().Header.Set(HeaderWorkspaceID, workspace.ID.String())
			c.Request().Header.Set(HeaderWorkspaceKey, workspace.WorkspaceKey)

			return next(c)
		}
	}
}
