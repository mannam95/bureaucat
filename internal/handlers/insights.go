package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/auth"
	"bereaucat/internal/insights"
	"bereaucat/internal/store"
)

// InsightsHandler serves the reconstruction-based reporting. Every endpoint is
// scoped to the viewer's memberships: the portfolio sums only their projects,
// and the per-project routes sit behind the project middleware like the rest
// of the API ("same scope, every answer").
type InsightsHandler struct {
	store  store.Querier
	engine *insights.Engine
	tz     *time.Location
}

func NewInsightsHandler(s store.Querier, engine *insights.Engine, tz *time.Location) *InsightsHandler {
	return &InsightsHandler{store: s, engine: engine, tz: tz}
}

// parseRange reads optional from/to date params (YYYY-MM-DD, reporting
// timezone). Zero times mean "engine default" (last 30 days).
func (h *InsightsHandler) parseRange(c *echo.Context) (time.Time, time.Time, error) {
	var from, to time.Time
	if s := strings.TrimSpace(c.QueryParam("from")); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, h.tz)
		if err != nil {
			return from, to, echo.NewHTTPError(http.StatusBadRequest, "invalid from date, want YYYY-MM-DD")
		}
		from = t
	}
	if s := strings.TrimSpace(c.QueryParam("to")); s != "" {
		t, err := time.ParseInLocation("2006-01-02", s, h.tz)
		if err != nil {
			return from, to, echo.NewHTTPError(http.StatusBadRequest, "invalid to date, want YYYY-MM-DD")
		}
		to = t
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return from, to, echo.NewHTTPError(http.StatusBadRequest, "to is before from")
	}
	return from, to, nil
}

// parseWorkspaceID reads the optional workspace_id param; invalid means a 400,
// absent means "across all the viewer's workspaces" (NULL filter).
func parseWorkspaceID(c *echo.Context) (pgtype.UUID, error) {
	var ws pgtype.UUID
	if s := strings.TrimSpace(c.QueryParam("workspace_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return ws, echo.NewHTTPError(http.StatusBadRequest, "invalid workspace_id")
		}
		ws = pgtype.UUID{Bytes: id, Valid: true}
	}
	return ws, nil
}

func (h *InsightsHandler) projectFromContext(c *echo.Context) (store.Project, error) {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return store.Project{}, echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}
	p, err := h.store.GetProjectByID(c.Request().Context(), projectID)
	if err != nil {
		return store.Project{}, echo.NewHTTPError(http.StatusNotFound, "project not found")
	}
	return p, nil
}

// Portfolio answers question 0 over the viewer's projects.
//
//	@Summary		Insights portfolio
//	@Description	Attention table and delivery overview across the viewer's projects.
//	@Tags			Insights
//	@Produce		json
//	@Param			from			query		string	false	"Window start (YYYY-MM-DD)"
//	@Param			to				query		string	false	"Window end (YYYY-MM-DD)"
//	@Param			workspace_id	query		string	false	"Limit to one workspace; absent means all the viewer's workspaces"
//	@Success		200				{object}	insights.Portfolio
//	@Security		BearerAuth
//	@Router			/insights/portfolio [get]
func (h *InsightsHandler) Portfolio(c *echo.Context) error {
	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}
	from, to, err := h.parseRange(c)
	if err != nil {
		return err
	}
	workspaceID, err := parseWorkspaceID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()

	rows, err := h.store.ListUserProjectsFiltered(ctx, store.ListUserProjectsFilteredParams{
		UserID: userID, Limit: 100, Offset: 0, WorkspaceID: workspaceID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list projects")
	}
	projects := make([]store.Project, 0, len(rows))
	icons := map[string]*string{}
	for _, r := range rows {
		projects = append(projects, store.Project{
			ID: r.ID, ProjectKey: r.ProjectKey, Name: r.Name,
			CreatedAt: r.CreatedAt, WorkspaceID: r.WorkspaceID,
		})
		icons[r.ProjectKey] = pgtypeUUIDToURL(r.IconID)
	}

	portfolio, err := h.engine.Portfolio(ctx, projects, from, to)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compute portfolio")
	}
	for i := range portfolio.Rows {
		portfolio.Rows[i].IconURL = icons[portfolio.Rows[i].ProjectKey]
	}
	return c.JSON(http.StatusOK, portfolio)
}

// ProjectReport answers questions 1-3 for one project.
//
//	@Summary		Insights project report
//	@Description	The project report: cycle or date-range delivery, people, health.
//	@Tags			Insights
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			cycle_id	query		string	false	"Cycle to report on; omitted means date range"
//	@Param			from		query		string	false	"Window start (YYYY-MM-DD)"
//	@Param			to			query		string	false	"Window end (YYYY-MM-DD)"
//	@Success		200			{object}	insights.ProjectReport
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/insights [get]
func (h *InsightsHandler) ProjectReport(c *echo.Context) error {
	project, err := h.projectFromContext(c)
	if err != nil {
		return err
	}
	opts, err := h.parseOptions(c)
	if err != nil {
		return err
	}
	rep, err := h.engine.ProjectReport(c.Request().Context(), project, opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compute report")
	}
	return c.JSON(http.StatusOK, rep)
}

// PersonReport answers questions 5-6 for one member of the project.
//
//	@Summary		Insights person report
//	@Description	One member's delivered work and current load inside the project.
//	@Tags			Insights
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			userId		path		string	true	"User ID"
//	@Success		200			{object}	insights.PersonReport
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/insights/person/{userId} [get]
func (h *InsightsHandler) PersonReport(c *echo.Context) error {
	project, err := h.projectFromContext(c)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}
	opts, err := h.parseOptions(c)
	if err != nil {
		return err
	}
	rep, err := h.engine.PersonReport(c.Request().Context(), project, userID, opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compute person report")
	}
	return c.JSON(http.StatusOK, rep)
}

// Evidence resolves one metric into its exact population; format=csv exports
// the same rows.
//
//	@Summary		Insights evidence
//	@Description	The exact population behind one reported number, with its computation rule.
//	@Tags			Insights
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			metric		query		string	true	"Metric key (delivered, planned, added, descoped, carried, cancelled, archived, completed, created, original_plan, person_delivered, person_open, open_state, priority_rank, aging, overdue, due_soon, blocked, day)"
//	@Param			format		query		string	false	"csv for a CSV export of the rows"
//	@Success		200			{object}	insights.Evidence
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/insights/evidence [get]
func (h *InsightsHandler) Evidence(c *echo.Context) error {
	project, err := h.projectFromContext(c)
	if err != nil {
		return err
	}
	opts, err := h.parseOptions(c)
	if err != nil {
		return err
	}
	params := insights.EvidenceParams{
		Metric:  strings.TrimSpace(c.QueryParam("metric")),
		CycleID: opts.CycleID,
		From:    opts.From,
		To:      opts.To,
		Day:     strings.TrimSpace(c.QueryParam("day")),
	}
	if params.Metric == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "metric is required")
	}
	if s := strings.TrimSpace(c.QueryParam("user_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
		}
		params.UserID = &id
	}
	if s := strings.TrimSpace(c.QueryParam("state_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid state_id")
		}
		params.StateID = &id
	}
	if s := strings.TrimSpace(c.QueryParam("rank")); s != "" {
		rank, err := strconv.Atoi(s)
		if err != nil || rank < 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid rank")
		}
		params.Rank = &rank
	}

	ev, err := h.engine.Evidence(c.Request().Context(), project, params)
	if err != nil {
		if strings.Contains(err.Error(), "unknown metric") || strings.Contains(err.Error(), "needs a") {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compute evidence")
	}

	if strings.EqualFold(c.QueryParam("format"), "csv") {
		return h.evidenceCSV(c, ev)
	}
	return c.JSON(http.StatusOK, ev)
}

func (h *InsightsHandler) parseOptions(c *echo.Context) (insights.ReportOptions, error) {
	var opts insights.ReportOptions
	if s := strings.TrimSpace(c.QueryParam("cycle_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return opts, echo.NewHTTPError(http.StatusBadRequest, "invalid cycle_id")
		}
		opts.CycleID = &id
	}
	from, to, err := h.parseRange(c)
	if err != nil {
		return opts, err
	}
	opts.From, opts.To = from, to
	return opts, nil
}

// evidenceCSV streams exactly the selected population — the same rows the
// drawer shows, never more.
func (h *InsightsHandler) evidenceCSV(c *echo.Context, ev *insights.Evidence) error {
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	res.Header().Set(echo.HeaderContentDisposition,
		fmt.Sprintf("attachment; filename=%q", "insights-"+ev.Metric+".csv"))
	res.WriteHeader(http.StatusOK)

	w := csv.NewWriter(res)
	header := []string{"key", "title", "parent", "state", "state_type", "priority",
		"complexity", "completed_at", "marked_complete_by", "credits", "assignees",
		"due", "entered_cycle", "age_days"}
	if err := w.Write(header); err != nil {
		return err
	}
	users := func(list []insights.UserRef) string {
		names := make([]string, len(list))
		for i, u := range list {
			names[i] = u.Username
		}
		return strings.Join(names, "; ")
	}
	for _, r := range ev.Rows {
		cx := "not assessed"
		if r.Assessed {
			cx = strconv.Itoa(r.Complexity)
		}
		completedAt, markedBy, due, age := "", "", "", ""
		if r.CompletedAt != nil {
			completedAt = r.CompletedAt.In(h.tz).Format(time.RFC3339)
		}
		if r.MarkedCompleteBy != nil {
			markedBy = r.MarkedCompleteBy.Username
		}
		if r.Due != nil {
			due = r.Due.In(h.tz).Format("2006-01-02")
		}
		if r.AgeDays > 0 {
			age = strconv.Itoa(r.AgeDays)
		}
		prio := ""
		if r.Priority != nil {
			prio = r.Priority.Name
		}
		if err := w.Write([]string{
			r.Key, r.Title, r.ParentKey, r.State.Name, r.State.Type, prio,
			cx, completedAt, markedBy, users(r.Credits), users(r.Assignees),
			due, r.EnteredCycle, age,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
