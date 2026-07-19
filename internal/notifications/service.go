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

	"bereaucat/internal/store"
)

// DefaultWindow is the coalescing window: activity on the same task within this
// duration is merged into the recipient's existing notification.
const DefaultWindow = 15 * time.Minute

// Service fans out task activity to per-user notification rows.
type Service struct {
	store  store.Querier
	window time.Duration
}

// NewService creates a notifications service with the default 15-minute window.
func NewService(s store.Querier) *Service {
	return &Service{store: s, window: DefaultWindow}
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
