package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/auth"
	"bereaucat/internal/store"
)

// PageHandler handles project documentation page endpoints.
type PageHandler struct {
	store store.Querier
}

// NewPageHandler creates a new page handler.
func NewPageHandler(s store.Querier) *PageHandler {
	return &PageHandler{store: s}
}

// PageResponse represents a page in API responses.
type PageResponse struct {
	ID               uuid.UUID `json:"id"`
	ProjectKey       string    `json:"project_key"`
	PageNumber       int       `json:"page_number"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	CreatedBy        uuid.UUID `json:"created_by"`
	CreatorUsername  string    `json:"creator_username"`
	CreatorFirstName string    `json:"creator_first_name"`
	CreatorLastName  string    `json:"creator_last_name"`
	CreatorAvatarURL *string   `json:"creator_avatar_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PageListItem represents a page in list responses (content omitted).
type PageListItem struct {
	ID               uuid.UUID `json:"id"`
	PageNumber       int       `json:"page_number"`
	Title            string    `json:"title"`
	CreatedBy        uuid.UUID `json:"created_by"`
	CreatorUsername  string    `json:"creator_username"`
	CreatorFirstName string    `json:"creator_first_name"`
	CreatorLastName  string    `json:"creator_last_name"`
	CreatorAvatarURL *string   `json:"creator_avatar_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreatePageRequest represents the request to create a page.
type CreatePageRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// UpdatePageRequest represents the request to update a page.
type UpdatePageRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// ListPages returns all pages for a project.
//
//	@Summary		List pages
//	@Description	Returns all documentation pages for a project (newest updated first).
//	@Tags			Pages
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			q			query		string	false	"Search over page title and content"
//	@Success		200			{array}		PageListItem
//	@Failure		500			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/pages [get]
func (h *PageHandler) ListPages(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}

	ctx := c.Request().Context()

	var search pgtype.Text
	if q := strings.TrimSpace(c.QueryParam("q")); q != "" {
		search = pgtype.Text{String: q, Valid: true}
	}

	pages, err := h.store.ListProjectPages(ctx, store.ListProjectPagesParams{
		ProjectID: projectID,
		Search:    search,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list pages")
	}

	out := make([]PageListItem, len(pages))
	for i, p := range pages {
		out[i] = PageListItem{
			ID:               p.ID,
			PageNumber:       int(p.PageNumber),
			Title:            p.Title,
			CreatedBy:        p.CreatedBy,
			CreatorUsername:  p.CreatorUsername,
			CreatorFirstName: p.CreatorFirstName,
			CreatorLastName:  p.CreatorLastName,
			CreatorAvatarURL: textToStringPtr(p.CreatorAvatarUrl),
			CreatedAt:        p.CreatedAt.Time,
			UpdatedAt:        p.UpdatedAt.Time,
		}
	}

	return c.JSON(http.StatusOK, out)
}

// CreatePage creates a new page.
//
//	@Summary		Create page
//	@Description	Create a new documentation page in the project.
//	@Tags			Pages
//	@Accept			json
//	@Produce		json
//	@Param			projectKey	path		string				true	"Project key"
//	@Param			body		body		CreatePageRequest	true	"Page details"
//	@Success		201			{object}	PageResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/pages [post]
func (h *PageHandler) CreatePage(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}

	projectKey := c.Request().Header.Get(auth.HeaderProjectKey)

	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}

	var req CreatePageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}

	ctx := c.Request().Context()

	nextNumber, err := h.store.GetNextPageNumber(ctx, projectID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get next page number")
	}

	page, err := h.store.CreatePage(ctx, store.CreatePageParams{
		ProjectID:  projectID,
		PageNumber: int32(nextNumber),
		Title:      req.Title,
		Content:    req.Content,
		CreatedBy:  userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create page")
	}

	// Read back with creator info for a consistent response shape.
	full, err := h.store.GetPageByProjectAndNumber(ctx, store.GetPageByProjectAndNumberParams{
		ProjectID:  projectID,
		PageNumber: page.PageNumber,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get page")
	}

	return c.JSON(http.StatusCreated, PageResponse{
		ID:               full.ID,
		ProjectKey:       projectKey,
		PageNumber:       int(full.PageNumber),
		Title:            full.Title,
		Content:          full.Content,
		CreatedBy:        full.CreatedBy,
		CreatorUsername:  full.CreatorUsername,
		CreatorFirstName: full.CreatorFirstName,
		CreatorLastName:  full.CreatorLastName,
		CreatorAvatarURL: textToStringPtr(full.CreatorAvatarUrl),
		CreatedAt:        full.CreatedAt.Time,
		UpdatedAt:        full.UpdatedAt.Time,
	})
}

// GetPage returns page details by page number.
//
//	@Summary		Get page
//	@Description	Returns page details by page number.
//	@Tags			Pages
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			pageNum		path		int		true	"Page number"
//	@Success		200			{object}	PageResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/pages/{pageNum} [get]
func (h *PageHandler) GetPage(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}

	projectKey := c.Request().Header.Get(auth.HeaderProjectKey)

	pageNum, err := strconv.Atoi(c.Param("pageNum"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid page number")
	}

	ctx := c.Request().Context()

	page, err := h.store.GetPageByProjectAndNumber(ctx, store.GetPageByProjectAndNumberParams{
		ProjectID:  projectID,
		PageNumber: int32(pageNum),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}

	return c.JSON(http.StatusOK, PageResponse{
		ID:               page.ID,
		ProjectKey:       projectKey,
		PageNumber:       int(page.PageNumber),
		Title:            page.Title,
		Content:          page.Content,
		CreatedBy:        page.CreatedBy,
		CreatorUsername:  page.CreatorUsername,
		CreatorFirstName: page.CreatorFirstName,
		CreatorLastName:  page.CreatorLastName,
		CreatorAvatarURL: textToStringPtr(page.CreatorAvatarUrl),
		CreatedAt:        page.CreatedAt.Time,
		UpdatedAt:        page.UpdatedAt.Time,
	})
}

// UpdatePage updates a page.
//
//	@Summary		Update page
//	@Description	Update a page's title and/or content.
//	@Tags			Pages
//	@Accept			json
//	@Produce		json
//	@Param			projectKey	path		string				true	"Project key"
//	@Param			pageNum		path		int					true	"Page number"
//	@Param			body		body		UpdatePageRequest	true	"Fields to update"
//	@Success		200			{object}	PageResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/pages/{pageNum} [patch]
func (h *PageHandler) UpdatePage(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}

	projectKey := c.Request().Header.Get(auth.HeaderProjectKey)

	pageNum, err := strconv.Atoi(c.Param("pageNum"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid page number")
	}

	var req UpdatePageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()

	page, err := h.store.GetPageByProjectAndNumber(ctx, store.GetPageByProjectAndNumberParams{
		ProjectID:  projectID,
		PageNumber: int32(pageNum),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}

	if _, err := h.store.UpdatePage(ctx, store.UpdatePageParams{
		ID:      page.ID,
		Title:   stringToPgtypeText(req.Title),
		Content: stringToPgtypeText(req.Content),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update page")
	}

	full, err := h.store.GetPageByProjectAndNumber(ctx, store.GetPageByProjectAndNumberParams{
		ProjectID:  projectID,
		PageNumber: int32(pageNum),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get page")
	}

	return c.JSON(http.StatusOK, PageResponse{
		ID:               full.ID,
		ProjectKey:       projectKey,
		PageNumber:       int(full.PageNumber),
		Title:            full.Title,
		Content:          full.Content,
		CreatedBy:        full.CreatedBy,
		CreatorUsername:  full.CreatorUsername,
		CreatorFirstName: full.CreatorFirstName,
		CreatorLastName:  full.CreatorLastName,
		CreatorAvatarURL: textToStringPtr(full.CreatorAvatarUrl),
		CreatedAt:        full.CreatedAt.Time,
		UpdatedAt:        full.UpdatedAt.Time,
	})
}

// DeletePage soft-deletes a page.
//
//	@Summary		Delete page
//	@Description	Soft-delete a page.
//	@Tags			Pages
//	@Produce		json
//	@Param			projectKey	path		string	true	"Project key"
//	@Param			pageNum		path		int		true	"Page number"
//	@Success		200			{object}	MessageResponse
//	@Failure		400			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/projects/{projectKey}/pages/{pageNum} [delete]
func (h *PageHandler) DeletePage(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderProjectID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid project ID in context")
	}

	pageNum, err := strconv.Atoi(c.Param("pageNum"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid page number")
	}

	ctx := c.Request().Context()

	page, err := h.store.GetPageByProjectAndNumber(ctx, store.GetPageByProjectAndNumberParams{
		ProjectID:  projectID,
		PageNumber: int32(pageNum),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "page not found")
	}

	if err := h.store.SoftDeletePage(ctx, page.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete page")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "page deleted"})
}
