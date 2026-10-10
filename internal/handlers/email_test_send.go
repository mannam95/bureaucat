package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"bereaucat/internal/auth"
	"bereaucat/internal/notifier"
	"bereaucat/internal/store"
)

// Test emails: the super admin can send any notification template to a chosen
// user with sample data, to verify SMTP delivery and preview the template in a
// real inbox. SMTP stays env-configured — there is nothing to edit through the
// API, only to verify.

// testEmailEvents maps the API's template names to notifier events. The keys
// double as the template dropdown in the admin UI.
var testEmailEvents = map[string]notifier.EventType{
	"task_assigned":      notifier.EventTaskAssigned,
	"mentioned":          notifier.EventMentioned,
	"commented":          notifier.EventCommented,
	"state_changed":      notifier.EventStateChanged,
	"added_as_requester": notifier.EventAddedAsRequester,
	"added_as_watcher":   notifier.EventAddedAsWatcher,
	"activity":           notifier.EventActivity,
}

// requireSuperAdmin resolves the calling user and rejects anyone who is not
// the configured break-glass account. With SUPERADMIN_EMAIL unset there is no
// super admin, so the endpoints are refused for everyone.
func (h *AdminHandler) requireSuperAdmin(c *echo.Context) (store.GetUserByIDRow, error) {
	userID, err := uuid.Parse(c.Request().Header.Get(auth.HeaderUserID))
	if err != nil {
		return store.GetUserByIDRow{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
	}
	user, err := h.store.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		return store.GetUserByIDRow{}, echo.NewHTTPError(http.StatusUnauthorized, "user not found")
	}
	if !h.isSuperAdmin(user.Email) {
		return store.GetUserByIDRow{}, echo.NewHTTPError(http.StatusForbidden, "only the super admin can manage test emails")
	}
	return user, nil
}

// EmailStatusResponse reports whether env-configured SMTP is usable.
type EmailStatusResponse struct {
	Configured bool   `json:"configured"`
	From       string `json:"from,omitempty"`
}

// EmailStatus reports whether the instance can send email (super admin only).
//
//	@Summary		Email status
//	@Description	Whether env-configured SMTP is usable, and the From address. Super admin only.
//	@Tags			Admin - Email
//	@Produce		json
//	@Success		200	{object}	EmailStatusResponse
//	@Security		BearerAuth
//	@Router			/admin/email [get]
func (h *AdminHandler) EmailStatus(c *echo.Context) error {
	if _, err := h.requireSuperAdmin(c); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, EmailStatusResponse{
		Configured: h.smtp.Enabled(),
		From:       h.smtp.From,
	})
}

// SendTestEmailRequest picks the recipient and the template to exercise.
type SendTestEmailRequest struct {
	UserID string `json:"user_id"`
	Event  string `json:"event"`
}

// SendTestEmail sends one notification template with sample data to the chosen
// user, synchronously, so SMTP problems surface in the response. The
// recipient's email preferences are deliberately bypassed — this is a
// diagnostic triggered by a human, not an automated notification.
//
//	@Summary		Send test email
//	@Description	Send a notification template with sample data to a chosen user. Super admin only.
//	@Tags			Admin - Email
//	@Accept			json
//	@Produce		json
//	@Param			body	body		SendTestEmailRequest	true	"Recipient and template"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Security		BearerAuth
//	@Router			/admin/email/test [post]
func (h *AdminHandler) SendTestEmail(c *echo.Context) error {
	actor, err := h.requireSuperAdmin(c)
	if err != nil {
		return err
	}
	if !h.smtp.Enabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"email is not configured on this instance (set SMTP_HOST, SMTP_PORT and SMTP_FROM_ADDRESS)")
	}

	var req SendTestEmailRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	event, ok := testEmailEvents[req.Event]
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "unknown template")
	}
	recipientID, err := uuid.Parse(req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}

	ctx := c.Request().Context()
	recipient, err := h.store.GetUserByID(ctx, recipientID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "recipient not found")
	}

	// Links in real emails use APP_BASE_URL; mirror that, falling back to the
	// request's own origin so the test still renders sensible links without it.
	baseURL := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = requestBaseURL(c)
	}

	n := notifier.Notification{
		Event:       event,
		RecipientID: recipient.ID,
		ActorName:   strings.TrimSpace(actor.FirstName + " " + actor.LastName),
		ProjectKey:  "TEST",
		TaskNumber:  123,
		TaskTitle:   "A sample task to verify email delivery",
		BaseURL:     baseURL,
	}
	if err := notifier.NewEmailNotifier(h.smtp).Send(ctx, recipient.Email, n); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "SMTP send failed: "+err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"sent_to": recipient.Email})
}
