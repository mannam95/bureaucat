// Package notifications implements per-user, persisted in-app notifications with
// read/unread state and write-time coalescing. When several activities happen on
// the same task within a short window, they collapse into a single notification
// row so users are not spammed (max 1 notification per task per window).
package notifications

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bereaucat/internal/notifier"
	"bereaucat/internal/store"
)

// DefaultWindow is the coalescing window: activity on the same task within this
// duration is merged into the recipient's existing notification.
const DefaultWindow = 15 * time.Minute

// Service fans out task activity to per-user notification rows, and mirrors
// the fan-out to email for recipients who opted into the matching category.
type Service struct {
	store  store.Querier
	window time.Duration
	// emailer delivers category-gated emails; nil disables the email mirror.
	emailer *notifier.Service
	// baseURL builds absolute task links in fan-out emails (APP_BASE_URL).
	// Empty renders relative links.
	baseURL string
}

// NewService creates a notifications service with the default 15-minute window.
func NewService(s store.Querier, emailer *notifier.Service, baseURL string) *Service {
	return &Service{store: s, window: DefaultWindow, emailer: emailer, baseURL: baseURL}
}

// emailEventFor maps an activity type to the email event mirrored to opted-in
// participants. Zero value = no email: comments, assignments, mentions, and
// requester/watcher additions are covered by direct, recipient-specific sends
// at their handler sites, and task_created would only duplicate those.
func emailEventFor(activityType string) notifier.EventType {
	switch activityType {
	case "state_changed":
		return notifier.EventStateChanged
	case "task_updated", "task_moved", "task_deleted",
		"label_added", "label_removed",
		"attachment_added", "attachment_removed",
		"cycle_added", "cycle_removed",
		"module_added", "module_removed",
		"assignee_removed", "originator_removed", "watcher_removed":
		return notifier.EventActivity
	default:
		return ""
	}
}

// EnqueueForActivity creates (or coalesces into) a notification for every
// participant of the task except the actor. It is best-effort: failures are
// logged and never propagated, so notification delivery can never break the
// underlying request.
func (s *Service) EnqueueForActivity(ctx context.Context, taskID uuid.UUID, activityType string, actorID uuid.UUID, commentID *uuid.UUID) {
	participants, err := s.store.ListTaskParticipants(ctx, taskID)
	if err != nil {
		log.Printf("notifications: failed to list participants for task %s: %v", taskID, err)
		return
	}

	for _, recipientID := range participants {
		s.enqueue(ctx, recipientID, taskID, activityType, actorID, commentID)
	}

	s.mirrorToEmail(ctx, taskID, activityType, actorID, participants)
}

// mirrorToEmail sends the category-gated email counterpart of an activity to
// every participant except the actor. Task and actor details are fetched once;
// any failure quietly skips the mirror (in-app delivery already happened).
func (s *Service) mirrorToEmail(ctx context.Context, taskID uuid.UUID, activityType string, actorID uuid.UUID, participants []uuid.UUID) {
	if s.emailer == nil {
		return
	}
	event := emailEventFor(activityType)
	if event == "" {
		return
	}

	task, err := s.store.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}
	actorName := ""
	if actor, err := s.store.GetUserByID(ctx, actorID); err == nil {
		actorName = actor.FirstName + " " + actor.LastName
		if actorName == " " {
			actorName = actor.Username
		}
	}

	for _, recipientID := range participants {
		if recipientID == actorID {
			continue
		}
		s.emailer.NotifyEmail(ctx, notifier.Notification{
			Event:       event,
			RecipientID: recipientID,
			ActorName:   actorName,
			ProjectKey:  task.ProjectKey,
			TaskNumber:  int(task.TaskNumber),
			TaskTitle:   task.Title,
			BaseURL:     s.baseURL,
		})
	}
}

// EnqueueForUser creates (or coalesces into) a notification for a single explicit
// recipient, bypassing the participant list. Used for @mentions, where the
// mentioned user may not be a participant of the task.
func (s *Service) EnqueueForUser(ctx context.Context, recipientID, taskID uuid.UUID, activityType string, actorID uuid.UUID, commentID *uuid.UUID) {
	s.enqueue(ctx, recipientID, taskID, activityType, actorID, commentID)
}

// enqueue creates or coalesces one recipient's notification, using the same
// write-time coalescing window as the activity fan-out. The actor is never
// notified of their own action.
func (s *Service) enqueue(ctx context.Context, recipientID, taskID uuid.UUID, activityType string, actorID uuid.UUID, commentID *uuid.UUID) {
	if recipientID == actorID {
		return
	}

	cutoff := pgtype.Timestamptz{Time: time.Now().Add(-s.window), Valid: true}
	commentRef := pgtype.UUID{Valid: false}
	if commentID != nil {
		commentRef = pgtype.UUID{Bytes: *commentID, Valid: true}
	}

	open, err := s.store.GetOpenNotification(ctx, store.GetOpenNotificationParams{
		RecipientID: recipientID,
		TaskID:      taskID,
		Cutoff:      cutoff,
	})
	if err == nil {
		// An open notification exists within the window: merge into it.
		if err := s.store.CoalesceNotification(ctx, store.CoalesceNotificationParams{
			ID:           open.ID,
			ActivityType: activityType,
			ActorID:      actorID,
			CommentID:    commentRef,
		}); err != nil {
			log.Printf("notifications: failed to coalesce notification %s: %v", open.ID, err)
		}
		return
	}

	// No open notification (or lookup miss): create a fresh one.
	if _, err := s.store.CreateNotification(ctx, store.CreateNotificationParams{
		RecipientID:  recipientID,
		TaskID:       taskID,
		ActivityType: activityType,
		ActorID:      actorID,
		CommentID:    commentRef,
	}); err != nil {
		log.Printf("notifications: failed to create notification for user %s on task %s: %v", recipientID, taskID, err)
	}
}
