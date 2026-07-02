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

// WorkspaceHandler handles workspace-related endpoints. Workspaces are an
// organizing layer above projects: every project belongs to exactly one
// workspace. Membership governs visibility only (no per-workspace roles), and
// workspace management (create/update/delete/members) is gated to global admins.
type WorkspaceHandler struct {
	store store.Querier
}

// NewWorkspaceHandler creates a new workspace handler.
func NewWorkspaceHandler(store store.Querier) *WorkspaceHandler {
	return &WorkspaceHandler{
		store: store,
	}
}

// WorkspaceResponse represents a workspace in API responses.
type WorkspaceResponse struct {
	ID           uuid.UUID `json:"id"`
	WorkspaceKey string    `json:"workspace_key"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	CreatedBy    uuid.UUID `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateWorkspaceRequest represents the request to create a workspace.
type CreateWorkspaceRequest struct {
	WorkspaceKey string  `json:"workspace_key"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
}

// UpdateWorkspaceRequest represents the request to update a workspace.
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// PaginatedWorkspacesResponse represents a paginated list of workspaces.
type PaginatedWorkspacesResponse struct {
	Workspaces []WorkspaceResponse `json:"workspaces"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	TotalPages int                 `json:"total_pages"`
}

// WorkspaceMemberResponse represents a workspace member in API responses.
type WorkspaceMemberResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
}

// AddWorkspaceMemberRequest represents the request to add a workspace member.
type AddWorkspaceMemberRequest struct {
	UserID string `json:"user_id"`
}

// ListWorkspaces returns paginated workspaces with optional search.
//
//	@Summary		List workspaces
//	@Description	Returns a paginated list of workspaces. Admins see all; others see only their memberships.
//	@Tags			Workspaces
//	@Produce		json
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(20)
//	@Param			search		query		string	false	"Search by name"
//	@Success		200			{object}	PaginatedWorkspacesResponse
//	@Failure		500			{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces [get]
func (h *WorkspaceHandler) ListWorkspaces(c *echo.Context) error {
	userIDStr := c.Request().Header.Get(auth.HeaderUserID)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	search := strings.TrimSpace(c.QueryParam("search"))
	searchParam := pgtype.Text{}
	if search != "" {
		searchParam = pgtype.Text{String: search, Valid: true}
	}

	ctx := c.Request().Context()
	userType := c.Request().Header.Get(auth.HeaderUserType)

	var total int64
	var workspaceResponses []WorkspaceResponse

	if userType == "admin" {
		total, err = h.store.CountAllWorkspacesFiltered(ctx, searchParam)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count workspaces")
		}
		workspaces, err := h.store.ListAllWorkspacesFiltered(ctx, store.ListAllWorkspacesFilteredParams{
			Limit:  int32(perPage),
			Offset: int32(offset),
			Search: searchParam,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workspaces")
		}
		workspaceResponses = make([]WorkspaceResponse, len(workspaces))
		for i, w := range workspaces {
			workspaceResponses[i] = WorkspaceResponse{
				ID:           w.ID,
				WorkspaceKey: w.WorkspaceKey,
				Name:         w.Name,
				Description:  textToStringPtr(w.Description),
				CreatedBy:    w.CreatedBy,
				CreatedAt:    w.CreatedAt.Time,
				UpdatedAt:    w.UpdatedAt.Time,
			}
		}
	} else {
		total, err = h.store.CountUserWorkspacesFiltered(ctx, store.CountUserWorkspacesFilteredParams{
			UserID: userID,
			Search: searchParam,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count workspaces")
		}
		workspaces, err := h.store.ListUserWorkspacesFiltered(ctx, store.ListUserWorkspacesFilteredParams{
			UserID: userID,
			Limit:  int32(perPage),
			Offset: int32(offset),
			Search: searchParam,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workspaces")
		}
		workspaceResponses = make([]WorkspaceResponse, len(workspaces))
		for i, w := range workspaces {
			workspaceResponses[i] = WorkspaceResponse{
				ID:           w.ID,
				WorkspaceKey: w.WorkspaceKey,
				Name:         w.Name,
				Description:  textToStringPtr(w.Description),
				CreatedBy:    w.CreatedBy,
				CreatedAt:    w.CreatedAt.Time,
				UpdatedAt:    w.UpdatedAt.Time,
			}
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(http.StatusOK, PaginatedWorkspacesResponse{
		Workspaces: workspaceResponses,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

// CreateWorkspace creates a new workspace and adds the creator as a member.
// Requires global admin.
//
//	@Summary		Create workspace
//	@Description	Create a new workspace. Creator is added as a member. Requires admin.
//	@Tags			Workspaces
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateWorkspaceRequest	true	"Workspace details"
//	@Success		201		{object}	WorkspaceResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces [post]
func (h *WorkspaceHandler) CreateWorkspace(c *echo.Context) error {
	userIDStr := c.Request().Header.Get(auth.HeaderUserID)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}

	var req CreateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.WorkspaceKey == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_key and name are required")
	}

	// Validate workspace key format (alphanumeric, uppercase, 2-10 chars).
	req.WorkspaceKey = strings.ToUpper(req.WorkspaceKey)
	if len(req.WorkspaceKey) < 2 || len(req.WorkspaceKey) > 10 {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_key must be 2-10 characters")
	}
	for _, r := range req.WorkspaceKey {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace_key must be alphanumeric")
		}
	}

	ctx := c.Request().Context()

	exists, err := h.store.WorkspaceKeyExists(ctx, req.WorkspaceKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check workspace key")
	}
	if exists {
		return echo.NewHTTPError(http.StatusConflict, "workspace key already exists")
	}

	workspace, err := h.store.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		WorkspaceKey: req.WorkspaceKey,
		Name:         req.Name,
		Description:  stringToPgtypeText(req.Description),
		CreatedBy:    userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create workspace")
	}

	// Add creator as a member so it shows up in their workspace switcher.
	_, err = h.store.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: workspace.ID,
		UserID:      userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add creator as member")
	}

	return c.JSON(http.StatusCreated, WorkspaceResponse{
		ID:           workspace.ID,
		WorkspaceKey: workspace.WorkspaceKey,
		Name:         workspace.Name,
		Description:  textToStringPtr(workspace.Description),
		CreatedBy:    workspace.CreatedBy,
		CreatedAt:    workspace.CreatedAt.Time,
		UpdatedAt:    workspace.UpdatedAt.Time,
	})
}

// GetWorkspace returns workspace details.
//
//	@Summary		Get workspace
//	@Description	Returns workspace details by key.
//	@Tags			Workspaces
//	@Produce		json
//	@Param			workspaceKey	path		string	true	"Workspace key"
//	@Success		200				{object}	WorkspaceResponse
//	@Failure		404				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey} [get]
func (h *WorkspaceHandler) GetWorkspace(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	ctx := c.Request().Context()

	workspace, err := h.store.GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "workspace not found")
	}

	return c.JSON(http.StatusOK, WorkspaceResponse{
		ID:           workspace.ID,
		WorkspaceKey: workspace.WorkspaceKey,
		Name:         workspace.Name,
		Description:  textToStringPtr(workspace.Description),
		CreatedBy:    workspace.CreatedBy,
		CreatedAt:    workspace.CreatedAt.Time,
		UpdatedAt:    workspace.UpdatedAt.Time,
	})
}

// UpdateWorkspace updates a workspace. Requires global admin.
//
//	@Summary		Update workspace
//	@Description	Update workspace details. Requires admin.
//	@Tags			Workspaces
//	@Accept			json
//	@Produce		json
//	@Param			workspaceKey	path		string					true	"Workspace key"
//	@Param			body			body		UpdateWorkspaceRequest	true	"Fields to update"
//	@Success		200				{object}	WorkspaceResponse
//	@Failure		400				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey} [patch]
func (h *WorkspaceHandler) UpdateWorkspace(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	var req UpdateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()

	workspace, err := h.store.UpdateWorkspace(ctx, store.UpdateWorkspaceParams{
		ID:          workspaceID,
		Name:        stringToPgtypeText(req.Name),
		Description: stringToPgtypeText(req.Description),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update workspace")
	}

	return c.JSON(http.StatusOK, WorkspaceResponse{
		ID:           workspace.ID,
		WorkspaceKey: workspace.WorkspaceKey,
		Name:         workspace.Name,
		Description:  textToStringPtr(workspace.Description),
		CreatedBy:    workspace.CreatedBy,
		CreatedAt:    workspace.CreatedAt.Time,
		UpdatedAt:    workspace.UpdatedAt.Time,
	})
}

// DeleteWorkspace soft deletes a workspace. Requires global admin.
//
//	@Summary		Delete workspace
//	@Description	Soft-delete a workspace. Requires admin.
//	@Tags			Workspaces
//	@Produce		json
//	@Param			workspaceKey	path		string	true	"Workspace key"
//	@Success		200				{object}	MessageResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey} [delete]
func (h *WorkspaceHandler) DeleteWorkspace(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	ctx := c.Request().Context()

	if err := h.store.SoftDeleteWorkspace(ctx, workspaceID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "workspace deleted"})
}

// ListMembers returns workspace members.
//
//	@Summary		List workspace members
//	@Description	Returns all members of a workspace.
//	@Tags			Workspace Members
//	@Produce		json
//	@Param			workspaceKey	path		string	true	"Workspace key"
//	@Success		200				{array}		WorkspaceMemberResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey}/members [get]
func (h *WorkspaceHandler) ListMembers(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	ctx := c.Request().Context()

	members, err := h.store.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list members")
	}

	memberResponses := make([]WorkspaceMemberResponse, len(members))
	for i, m := range members {
		memberResponses[i] = WorkspaceMemberResponse{
			ID:        m.ID,
			UserID:    m.UserID,
			Username:  m.Username,
			Email:     m.Email,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			AvatarURL: textToStringPtr(m.AvatarUrl),
			JoinedAt:  m.JoinedAt.Time,
		}
	}

	return c.JSON(http.StatusOK, memberResponses)
}

// SearchUsers searches the user directory so an admin can pick someone to add
// as a workspace member.
//
//	@Summary		Search users for workspace membership
//	@Description	Search users by name/username/email. Requires admin.
//	@Tags			Workspace Members
//	@Produce		json
//	@Param			workspaceKey	path	string	true	"Workspace key"
//	@Param			q				query	string	false	"Search query"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey}/members/search [get]
func (h *WorkspaceHandler) SearchUsers(c *echo.Context) error {
	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{"users": []UserResponse{}})
	}

	ctx := c.Request().Context()

	results, err := h.store.SearchUsersPaginated(ctx, store.SearchUsersPaginatedParams{
		Column1: pgtype.Text{String: query, Valid: true},
		Limit:   20,
		Offset:  0,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to search users")
	}

	users := make([]UserResponse, len(results))
	for i, u := range results {
		users[i] = UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			UserType:  u.UserType,
			CreatedAt: u.CreatedAt.Time,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"users": users})
}

// AddMember adds a user to a workspace. Requires global admin.
//
//	@Summary		Add workspace member
//	@Description	Add a user as a member of the workspace. Requires admin.
//	@Tags			Workspace Members
//	@Accept			json
//	@Produce		json
//	@Param			workspaceKey	path		string						true	"Workspace key"
//	@Param			body			body		AddWorkspaceMemberRequest	true	"Member details"
//	@Success		201				{object}	WorkspaceMemberResponse
//	@Failure		400				{object}	ErrorResponse
//	@Failure		409				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey}/members [post]
func (h *WorkspaceHandler) AddMember(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	var req AddWorkspaceMemberRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}

	ctx := c.Request().Context()

	isMember, err := h.store.IsWorkspaceMember(ctx, store.IsWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
	}
	if isMember {
		return echo.NewHTTPError(http.StatusConflict, "user is already a member")
	}

	member, err := h.store.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add member")
	}

	fullMember, err := h.store.GetWorkspaceMember(ctx, store.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get member info")
	}

	return c.JSON(http.StatusCreated, WorkspaceMemberResponse{
		ID:        member.ID,
		UserID:    fullMember.UserID,
		Username:  fullMember.Username,
		Email:     fullMember.Email,
		FirstName: fullMember.FirstName,
		LastName:  fullMember.LastName,
		AvatarURL: textToStringPtr(fullMember.AvatarUrl),
		JoinedAt:  member.JoinedAt.Time,
	})
}

// RemoveMember removes a member from a workspace. Requires global admin.
//
//	@Summary		Remove workspace member
//	@Description	Remove a member from the workspace. Requires admin.
//	@Tags			Workspace Members
//	@Produce		json
//	@Param			workspaceKey	path		string	true	"Workspace key"
//	@Param			userId			path		string	true	"User ID"
//	@Success		200				{object}	MessageResponse
//	@Failure		400				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/workspaces/{workspaceKey}/members/{userId} [delete]
func (h *WorkspaceHandler) RemoveMember(c *echo.Context) error {
	workspaceIDStr := c.Request().Header.Get(auth.HeaderWorkspaceID)
	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid workspace ID in context")
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	ctx := c.Request().Context()

	if err := h.store.RemoveWorkspaceMember(ctx, store.RemoveWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove member")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "member removed"})
}
