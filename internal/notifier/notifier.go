package notifier

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// EventType represents the type of notification event.
type EventType string

const (
	EventTaskAssigned     EventType = "task_assigned"
	EventMentioned        EventType = "mentioned"
	EventCommented        EventType = "commented"
	EventStateChanged     EventType = "state_changed"
	EventAddedAsRequester EventType = "added_as_requester"
	EventAddedAsWatcher   EventType = "added_as_watcher"
	// EventActivity is the generic bucket for other ticket updates (edits,
	// labels, dates, attachments, cycle/module moves).
	EventActivity EventType = "activity"
)

// EmailCategory groups events into the per-user email toggles stored in the
// notifications.email_events preference.
type EmailCategory string

const (
	CategoryAssigned      EmailCategory = "assigned"
	CategoryComments      EmailCategory = "comments"
	CategoryMentions      EmailCategory = "mentions"
	CategoryStatusChanges EmailCategory = "status_changes"
	CategoryRoles         EmailCategory = "roles"
	CategoryActivity      EmailCategory = "activity"
)

// Category maps an event to its email toggle.
func (e EventType) Category() EmailCategory {
	switch e {
	case EventTaskAssigned:
		return CategoryAssigned
	case EventCommented:
		return CategoryComments
	case EventMentioned:
		return CategoryMentions
	case EventStateChanged:
		return CategoryStatusChanges
	case EventAddedAsRequester, EventAddedAsWatcher:
		return CategoryRoles
	default:
		return CategoryActivity
	}
}

// Notification represents a single notification to deliver.
type Notification struct {
	Event       EventType
	RecipientID uuid.UUID // Bureaucat user ID to notify
	ActorName   string    // Who triggered the notification (e.g. "John Doe")
	ProjectKey  string    // e.g. "DEVOP"
	TaskNumber  int       // e.g. 123
	TaskTitle   string
	BaseURL     string // App base URL for building task links (e.g. "https://bureaucat.example.com")
}

// TaskURL returns the full URL to the task.
func (n Notification) TaskURL() string {
	return fmt.Sprintf("%s/projects/%s/tasks/%d", n.BaseURL, n.ProjectKey, n.TaskNumber)
}

// Notifier is the interface that all notification providers must implement.
type Notifier interface {
	// Name returns the provider name (e.g. "mattermost", "slack").
	Name() string
	// Send delivers a notification to the recipient identified by email.
	Send(ctx context.Context, recipientEmail string, notification Notification) error
}
