package notifier

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"bereaucat/internal/store"
)

// Service dispatches notifications to all enabled providers.
// It reads provider configuration from the database on each call,
// with a short cache to avoid excessive DB reads.
type Service struct {
	store store.Querier

	// staticProviders are always active (e.g. an env-configured email provider),
	// alongside the providers loaded dynamically from settings.
	staticProviders []Notifier

	mu          sync.RWMutex
	cachedAt    time.Time
	cachedProvs []Notifier
	cacheTTL    time.Duration
}

// NewService creates a notification service that dynamically loads providers from
// settings. Any static providers passed here are always included as well.
func NewService(s store.Querier, static ...Notifier) *Service {
	return &Service{
		store:           s,
		cacheTTL:        30 * time.Second,
		staticProviders: static,
	}
}

// Notify sends a notification to a single recipient via all enabled providers.
// Sends are dispatched asynchronously to avoid blocking the caller.
func (s *Service) Notify(ctx context.Context, n Notification) {
	providers := s.getProviders()
	if len(providers) == 0 {
		return
	}

	email, err := s.getRecipientEmail(ctx, n.RecipientID)
	if err != nil {
		log.Printf("notifier: failed to look up email for user %s: %v", n.RecipientID, err)
		return
	}

	// Email is opt-out per user and per category; other providers (e.g.
	// Mattermost) are not gated by these preferences. Check once and reuse for
	// every email provider.
	emailAllowed := s.EmailAllowed(ctx, n.RecipientID, n.Event.Category())

	for _, provider := range providers {
		if provider.Name() == "email" && !emailAllowed {
			continue
		}
		go func(p Notifier) {
			if err := p.Send(context.Background(), email, n); err != nil {
				log.Printf("notifier [%s]: failed to send to %s: %v", p.Name(), email, err)
			}
		}(provider)
	}
}

// Category defaults when the user has never touched the granular toggles:
// deliberate signals on, ambient ones off. Mirrors the preferences registry
// default for notifications.email_events.
var defaultEmailCategories = map[EmailCategory]bool{
	CategoryAssigned:      true,
	CategoryComments:      true,
	CategoryMentions:      true,
	CategoryStatusChanges: false,
	CategoryRoles:         false,
	CategoryActivity:      false,
}

// EmailAllowed reports whether the recipient wants email for this category:
// the master switch AND the category toggle. Missing rows or read errors fall
// back to the defaults, so a lookup hiccup never silently drops the deliberate
// signals (and never turns the ambient ones on).
func (s *Service) EmailAllowed(ctx context.Context, userID uuid.UUID, category EmailCategory) bool {
	master := true
	if row, err := s.store.GetGlobalPreference(ctx, store.GetGlobalPreferenceParams{
		UserID:        userID,
		PreferenceKey: "notifications.email_enabled",
	}); err == nil {
		var enabled bool
		if json.Unmarshal(row.Value, &enabled) == nil {
			master = enabled
		}
	}
	if !master {
		return false
	}

	enabled := defaultEmailCategories[category]
	if row, err := s.store.GetGlobalPreference(ctx, store.GetGlobalPreferenceParams{
		UserID:        userID,
		PreferenceKey: "notifications.email_events",
	}); err == nil {
		var m map[string]bool
		if json.Unmarshal(row.Value, &m) == nil {
			if v, ok := m[string(category)]; ok {
				enabled = v
			}
		}
	}
	return enabled
}

// NotifyEmail delivers via the email provider only, honoring the recipient's
// master and per-category toggles. Used by the activity fan-out, which must
// not ping chat providers (their behavior stays limited to the direct events).
func (s *Service) NotifyEmail(ctx context.Context, n Notification) {
	if !s.EmailAllowed(ctx, n.RecipientID, n.Event.Category()) {
		return
	}
	email, err := s.getRecipientEmail(ctx, n.RecipientID)
	if err != nil {
		log.Printf("notifier: failed to look up email for user %s: %v", n.RecipientID, err)
		return
	}
	for _, provider := range s.getProviders() {
		if provider.Name() != "email" {
			continue
		}
		go func(p Notifier) {
			if err := p.Send(context.Background(), email, n); err != nil {
				log.Printf("notifier [%s]: failed to send to %s: %v", p.Name(), email, err)
			}
		}(provider)
	}
}

// NotifyAll sends notifications to multiple recipients via all providers.
func (s *Service) NotifyAll(ctx context.Context, notifications []Notification) {
	for _, n := range notifications {
		s.Notify(ctx, n)
	}
}

// NotifyExcluding sends notifications to all recipients except the excluded user.
func (s *Service) NotifyExcluding(ctx context.Context, notifications []Notification, excludeUserID uuid.UUID) {
	for _, n := range notifications {
		if n.RecipientID != excludeUserID {
			s.Notify(ctx, n)
		}
	}
}

// InvalidateCache forces the next call to reload providers from the database.
func (s *Service) InvalidateCache() {
	s.mu.Lock()
	s.cachedAt = time.Time{}
	s.mu.Unlock()
}

func (s *Service) getProviders() []Notifier {
	s.mu.RLock()
	if !s.cachedAt.IsZero() && time.Since(s.cachedAt) < s.cacheTTL {
		provs := s.cachedProvs
		s.mu.RUnlock()
		return provs
	}
	s.mu.RUnlock()

	// Reload from DB
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if !s.cachedAt.IsZero() && time.Since(s.cachedAt) < s.cacheTTL {
		return s.cachedProvs
	}

	// Start with the always-on static providers (e.g. env-configured email).
	providers := append([]Notifier{}, s.staticProviders...)

	// Load Mattermost
	setting, err := s.store.GetSetting(context.Background(), "mattermost")
	if err == nil {
		var mm struct {
			Enabled   bool   `json:"enabled"`
			ServerURL string `json:"server_url"`
			BotToken  string `json:"bot_token"`
		}
		if json.Unmarshal(setting.Value, &mm) == nil && mm.Enabled && mm.ServerURL != "" && mm.BotToken != "" {
			providers = append(providers, NewMattermostNotifier(MattermostConfig{
				ServerURL: mm.ServerURL,
				BotToken:  mm.BotToken,
			}))
		}
	}

	s.cachedProvs = providers
	s.cachedAt = time.Now()
	return providers
}

func (s *Service) getRecipientEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}
