package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/auth"
	"bereaucat/internal/preferences"
	"bereaucat/internal/store"
)

// PreferencesHandler serves per-user preferences. Global preferences hang off
// the /me group; project preferences hang off the project group, which already
// enforces membership and injects the project ID header.
type PreferencesHandler struct {
	store store.Querier
}

func NewPreferencesHandler(s store.Querier) *PreferencesHandler {
	return &PreferencesHandler{store: s}
}

// PreferenceEntry is the API shape for one effective preference. `source` is
// "user" when it came from a stored row and "default" when it fell back to the
// registry. A default entry always reports revision 0.
type PreferenceEntry struct {
	Key          string          `json:"key"`
	Scope        string          `json:"scope"`
	Value        json.RawMessage `json:"value"`
	ValueVersion int             `json:"value_version"`
	Revision     int64           `json:"revision"`
	Source       string          `json:"source"`
}

// PreferencesResponse wraps the effective set for one scope.
type PreferencesResponse struct {
	Scope       string            `json:"scope"`
	Preferences []PreferenceEntry `json:"preferences"`
}

// WritePreferenceRequest is the body for PUT. `expected_revision` is the
// revision the caller last saw (0 for a first write); a mismatch yields 409.
type WritePreferenceRequest struct {
	Value            json.RawMessage `json:"value"`
	ValueVersion     int             `json:"value_version"`
	ExpectedRevision int64           `json:"expected_revision"`
}

func defaultEntry(d preferences.Definition) PreferenceEntry {
	return PreferenceEntry{
		Key:          d.Key,
		Scope:        string(d.Scope),
		Value:        d.Default,
		ValueVersion: d.ValueVersion,
		Revision:     0,
		Source:       "default",
	}
}

// GetGlobalPreferences returns every global preference, folding stored rows
// over registry defaults.
func (h *PreferencesHandler) GetGlobalPreferences(c *echo.Context) error {
	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}
	rows, err := h.store.ListGlobalPreferences(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load preferences")
	}
	stored := make(map[string]store.ListGlobalPreferencesRow, len(rows))
	for _, r := range rows {
		stored[r.PreferenceKey] = r
	}
	defs := preferences.ForScope(preferences.ScopeGlobal)
	out := make([]PreferenceEntry, 0, len(defs))
	for _, d := range defs {
		if r, ok := stored[d.Key]; ok {
			out = append(out, PreferenceEntry{
				Key:          d.Key,
				Scope:        string(preferences.ScopeGlobal),
				Value:        json.RawMessage(r.Value),
				ValueVersion: int(r.ValueVersion),
				Revision:     r.Revision,
				Source:       "user",
			})
			continue
		}
		out = append(out, defaultEntry(d))
	}
	return c.JSON(http.StatusOK, PreferencesResponse{
		Scope:       string(preferences.ScopeGlobal),
		Preferences: out,
	})
}

// GetProjectPreferences returns every project-scoped preference for the current
// project, folding stored rows over registry defaults.
func (h *PreferencesHandler) GetProjectPreferences(c *echo.Context) error {
	projectID, callerID, err := projectAndCaller(c)
	if err != nil {
		return err
	}
	rows, err := h.store.ListProjectPreferences(c.Request().Context(), store.ListProjectPreferencesParams{
		UserID:    callerID,
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load preferences")
	}
	stored := make(map[string]store.ListProjectPreferencesRow, len(rows))
	for _, r := range rows {
		stored[r.PreferenceKey] = r
	}
	defs := preferences.ForScope(preferences.ScopeProject)
	out := make([]PreferenceEntry, 0, len(defs))
	for _, d := range defs {
		if r, ok := stored[d.Key]; ok {
			out = append(out, PreferenceEntry{
				Key:          d.Key,
				Scope:        string(preferences.ScopeProject),
				Value:        json.RawMessage(r.Value),
				ValueVersion: int(r.ValueVersion),
				Revision:     r.Revision,
				Source:       "user",
			})
			continue
		}
		out = append(out, defaultEntry(d))
	}
	return c.JSON(http.StatusOK, PreferencesResponse{
		Scope:       string(preferences.ScopeProject),
		Preferences: out,
	})
}

// PutGlobalPreference creates or updates a global preference under optimistic
// concurrency.
func (h *PreferencesHandler) PutGlobalPreference(c *echo.Context) error {
	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}
	def, ok := preferences.Lookup(c.Param("key"))
	if !ok || def.Scope != preferences.ScopeGlobal {
		return echo.NewHTTPError(http.StatusNotFound, "unknown preference key")
	}
	var req WritePreferenceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := def.Check(preferences.ScopeGlobal, req.Value); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	valueVersion := req.ValueVersion
	if valueVersion <= 0 {
		valueVersion = def.ValueVersion
	}
	row, err := h.store.UpsertGlobalPreference(c.Request().Context(), store.UpsertGlobalPreferenceParams{
		UserID:           userID,
		PreferenceKey:    def.Key,
		Value:            []byte(req.Value),
		ValueVersion:     int32(valueVersion),
		ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusConflict, "preference was changed elsewhere; reload and retry")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save preference")
	}
	return c.JSON(http.StatusOK, PreferenceEntry{
		Key:          def.Key,
		Scope:        string(preferences.ScopeGlobal),
		Value:        json.RawMessage(row.Value),
		ValueVersion: int(row.ValueVersion),
		Revision:     row.Revision,
		Source:       "user",
	})
}

// PutProjectPreference creates or updates a project-scoped preference.
func (h *PreferencesHandler) PutProjectPreference(c *echo.Context) error {
	projectID, callerID, err := projectAndCaller(c)
	if err != nil {
		return err
	}
	def, ok := preferences.Lookup(c.Param("key"))
	if !ok || def.Scope != preferences.ScopeProject {
		return echo.NewHTTPError(http.StatusNotFound, "unknown preference key")
	}
	var req WritePreferenceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := def.Check(preferences.ScopeProject, req.Value); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	valueVersion := req.ValueVersion
	if valueVersion <= 0 {
		valueVersion = def.ValueVersion
	}
	row, err := h.store.UpsertProjectPreference(c.Request().Context(), store.UpsertProjectPreferenceParams{
		UserID:           callerID,
		PreferenceKey:    def.Key,
		ProjectID:        pgtype.UUID{Bytes: projectID, Valid: true},
		Value:            []byte(req.Value),
		ValueVersion:     int32(valueVersion),
		ExpectedRevision: req.ExpectedRevision,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusConflict, "preference was changed elsewhere; reload and retry")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save preference")
	}
	return c.JSON(http.StatusOK, PreferenceEntry{
		Key:          def.Key,
		Scope:        string(preferences.ScopeProject),
		Value:        json.RawMessage(row.Value),
		ValueVersion: int(row.ValueVersion),
		Revision:     row.Revision,
		Source:       "user",
	})
}

// DeleteGlobalPreference resets a global preference by deleting its row; the
// registry default is returned so the caller can adopt it without a refetch.
func (h *PreferencesHandler) DeleteGlobalPreference(c *echo.Context) error {
	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}
	def, ok := preferences.Lookup(c.Param("key"))
	if !ok || def.Scope != preferences.ScopeGlobal {
		return echo.NewHTTPError(http.StatusNotFound, "unknown preference key")
	}
	if err := h.store.DeleteGlobalPreference(c.Request().Context(), store.DeleteGlobalPreferenceParams{
		UserID:        userID,
		PreferenceKey: def.Key,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reset preference")
	}
	return c.JSON(http.StatusOK, defaultEntry(def))
}

// DeleteProjectPreference resets a project-scoped preference by deleting its row.
func (h *PreferencesHandler) DeleteProjectPreference(c *echo.Context) error {
	projectID, callerID, err := projectAndCaller(c)
	if err != nil {
		return err
	}
	def, ok := preferences.Lookup(c.Param("key"))
	if !ok || def.Scope != preferences.ScopeProject {
		return echo.NewHTTPError(http.StatusNotFound, "unknown preference key")
	}
	if err := h.store.DeleteProjectPreference(c.Request().Context(), store.DeleteProjectPreferenceParams{
		UserID:        callerID,
		PreferenceKey: def.Key,
		ProjectID:     pgtype.UUID{Bytes: projectID, Valid: true},
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reset preference")
	}
	return c.JSON(http.StatusOK, defaultEntry(def))
}
