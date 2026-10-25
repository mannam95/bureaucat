package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/features"
	"bereaucat/internal/store"
)

// Feature gates: the super admin decides who can use a gated feature (mode
// disabled/admins/everyone, plus grants for specific people). Only the
// break-glass account manages gates — ordinary admins cannot widen their own
// access. Enforcement lives in the features.Require middleware on the gated
// routes; these endpoints only manage the rules.

// FeatureGrantInfo is one specifically-granted user on a feature gate.
type FeatureGrantInfo struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// FeatureGateResponse is one gated feature with its mode and grants.
type FeatureGateResponse struct {
	Feature     string             `json:"feature"`
	Description string             `json:"description"`
	Mode        string             `json:"mode"`
	Grants      []FeatureGrantInfo `json:"grants"`
}

// gateResponse assembles the response for one registered feature.
func (h *AdminHandler) gateResponse(c *echo.Context, feature, mode string) (FeatureGateResponse, error) {
	rows, err := h.store.ListFeatureGrants(c.Request().Context(), feature)
	if err != nil {
		return FeatureGateResponse{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to list feature grants")
	}
	grants := make([]FeatureGrantInfo, len(rows))
	for i, r := range rows {
		grants[i] = FeatureGrantInfo{
			UserID:    r.UserID,
			Username:  r.Username,
			Email:     r.Email,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			AvatarURL: textToStringPtr(r.AvatarUrl),
			CreatedAt: r.CreatedAt.Time,
		}
	}
	return FeatureGateResponse{
		Feature:     feature,
		Description: features.Describe(feature),
		Mode:        mode,
		Grants:      grants,
	}, nil
}

// ListFeatureGates returns every registered feature with its mode and grants.
//
//	@Summary		List feature gates
//	@Description	Every registered gated feature with its mode and specific-user grants. Super admin only.
//	@Tags			Admin - Features
//	@Produce		json
//	@Success		200	{array}		FeatureGateResponse
//	@Failure		403	{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/admin/features [get]
func (h *AdminHandler) ListFeatureGates(c *echo.Context) error {
	if _, err := h.requireSuperAdmin(c); err != nil {
		return err
	}

	gates, err := h.store.ListFeatureGates(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list feature gates")
	}
	modeByFeature := map[string]string{}
	for _, g := range gates {
		modeByFeature[g.Feature] = g.Mode
	}

	// The registry drives the list: a registered feature missing its row shows
	// as disabled, a stale row for a removed feature is ignored.
	out := []FeatureGateResponse{}
	for _, f := range features.All() {
		mode, ok := modeByFeature[f]
		if !ok {
			mode = features.ModeDisabled
		}
		resp, err := h.gateResponse(c, f, mode)
		if err != nil {
			return err
		}
		out = append(out, resp)
	}
	return c.JSON(http.StatusOK, out)
}

// SetFeatureGateRequest is the body for PUT /admin/features/:feature.
type SetFeatureGateRequest struct {
	Mode string `json:"mode"`
}

// SetFeatureGate sets a feature's mode.
//
//	@Summary		Set feature gate mode
//	@Description	Sets who can use a feature: disabled, admins or everyone. Grants stay and remain additive. Super admin only.
//	@Tags			Admin - Features
//	@Accept			json
//	@Produce		json
//	@Param			feature	path		string					true	"Feature key"
//	@Param			request	body		SetFeatureGateRequest	true	"Mode"
//	@Success		200		{object}	FeatureGateResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/admin/features/{feature} [put]
func (h *AdminHandler) SetFeatureGate(c *echo.Context) error {
	caller, err := h.requireSuperAdmin(c)
	if err != nil {
		return err
	}

	feature := c.Param("feature")
	if !features.Known(feature) {
		return echo.NewHTTPError(http.StatusNotFound, "unknown feature")
	}
	var req SetFeatureGateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !features.ValidMode(req.Mode) {
		return echo.NewHTTPError(http.StatusBadRequest, "mode must be disabled, admins or everyone")
	}

	if err := h.store.SetFeatureGateMode(c.Request().Context(), store.SetFeatureGateModeParams{
		Feature:   feature,
		Mode:      req.Mode,
		UpdatedBy: pgtype.UUID{Bytes: caller.ID, Valid: true},
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update feature gate")
	}

	resp, err := h.gateResponse(c, feature, req.Mode)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// AddFeatureGrantRequest is the body for POST /admin/features/:feature/grants.
type AddFeatureGrantRequest struct {
	UserID string `json:"user_id"`
}

// AddFeatureGrant grants a feature to one user, on top of the gate's mode.
//
//	@Summary		Grant a feature to a user
//	@Description	Adds a specific-user grant on a feature gate; additive on top of the mode. Super admin only.
//	@Tags			Admin - Features
//	@Accept			json
//	@Produce		json
//	@Param			feature	path		string					true	"Feature key"
//	@Param			request	body		AddFeatureGrantRequest	true	"User to grant"
//	@Success		200		{object}	FeatureGateResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/admin/features/{feature}/grants [post]
func (h *AdminHandler) AddFeatureGrant(c *echo.Context) error {
	caller, err := h.requireSuperAdmin(c)
	if err != nil {
		return err
	}

	feature := c.Param("feature")
	if !features.Known(feature) {
		return echo.NewHTTPError(http.StatusNotFound, "unknown feature")
	}
	var req AddFeatureGrantRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	ctx := c.Request().Context()
	user, err := h.store.GetUserByID(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	// The gate row must exist for the grant's foreign key; a registry feature
	// that never had its mode saved gets its disabled row here.
	mode, err := h.store.GetFeatureGateMode(ctx, feature)
	if err != nil {
		mode = features.ModeDisabled
		if err := h.store.SetFeatureGateMode(ctx, store.SetFeatureGateModeParams{
			Feature:   feature,
			Mode:      mode,
			UpdatedBy: pgtype.UUID{Bytes: caller.ID, Valid: true},
		}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create feature gate")
		}
	}

	if err := h.store.AddFeatureGrant(ctx, store.AddFeatureGrantParams{
		Feature:   feature,
		UserID:    user.ID,
		GrantedBy: pgtype.UUID{Bytes: caller.ID, Valid: true},
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add feature grant")
	}

	resp, err := h.gateResponse(c, feature, mode)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// RemoveFeatureGrant removes one user's grant on a feature.
//
//	@Summary		Revoke a feature grant
//	@Description	Removes a specific-user grant from a feature gate. The user keeps access the mode gives them. Super admin only.
//	@Tags			Admin - Features
//	@Produce		json
//	@Param			feature	path		string	true	"Feature key"
//	@Param			userId	path		string	true	"User ID"
//	@Success		200		{object}	FeatureGateResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/admin/features/{feature}/grants/{userId} [delete]
func (h *AdminHandler) RemoveFeatureGrant(c *echo.Context) error {
	if _, err := h.requireSuperAdmin(c); err != nil {
		return err
	}

	feature := c.Param("feature")
	if !features.Known(feature) {
		return echo.NewHTTPError(http.StatusNotFound, "unknown feature")
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	ctx := c.Request().Context()
	if err := h.store.RemoveFeatureGrant(ctx, store.RemoveFeatureGrantParams{
		Feature: feature,
		UserID:  userID,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove feature grant")
	}

	mode, err := h.store.GetFeatureGateMode(ctx, feature)
	if err != nil {
		mode = features.ModeDisabled
	}
	resp, err := h.gateResponse(c, feature, mode)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}
