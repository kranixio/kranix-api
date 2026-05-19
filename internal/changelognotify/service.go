package changelognotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kranix-io/kranix-packages/types"
)

// EmailConfig configures SMTP delivery for changelog alerts.
type EmailConfig struct {
	Enabled  bool
	SMTPHost string
	SMTPPort string
	Username string
	Password string
	From     string
}

// Config configures changelog notification delivery.
type Config struct {
	Enabled bool
	Email   EmailConfig
}

// Service manages changelog subscriptions and breaking-change notifications.
type Service struct {
	cfg           Config
	mu            sync.RWMutex
	subscriptions map[string]*types.ChangelogSubscription
	httpClient    *http.Client
}

// New creates a changelog notification service.
func New(cfg Config) *Service {
	return &Service{
		cfg:           cfg,
		subscriptions: make(map[string]*types.ChangelogSubscription),
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether notifications are active.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// Subscribe registers a webhook or email listener.
func (s *Service) Subscribe(sub *types.ChangelogSubscription) (*types.ChangelogSubscription, error) {
	if sub == nil {
		return nil, fmt.Errorf("subscription required")
	}
	if sub.WebhookURL == "" && sub.Email == "" {
		return nil, fmt.Errorf("webhookUrl or email is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	sub.CreatedAt = time.Now().UTC()
	if !sub.Enabled {
		sub.Enabled = true
	}
	cp := *sub
	s.subscriptions[sub.ID] = &cp
	return &cp, nil
}

// ListSubscriptions returns all subscriptions.
func (s *Service) ListSubscriptions() []*types.ChangelogSubscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*types.ChangelogSubscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		cp := *sub
		out = append(out, &cp)
	}
	return out
}

// Unsubscribe removes a subscription.
func (s *Service) Unsubscribe(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscriptions[id]; !ok {
		return false
	}
	delete(s.subscriptions, id)
	return true
}

// NotifyBreakingRelease delivers alerts for breaking changelog entries.
func (s *Service) NotifyBreakingRelease(ctx context.Context, version string, entries []types.ChangelogEntry) types.ChangelogNotifyResult {
	result := types.ChangelogNotifyResult{Version: version}
	if !s.Enabled() {
		return result
	}

	var breaking []types.ChangelogEntry
	for _, e := range entries {
		if e.Breaking {
			breaking = append(breaking, e)
		}
	}
	if len(breaking) == 0 {
		return result
	}

	payload := types.ChangelogNotificationPayload{
		Version:         version,
		ReleasedAt:      time.Now().UTC(),
		BreakingChanges: breaking,
		AllChanges:      entries,
		Message:         fmt.Sprintf("Breaking API changes released in %s", version),
	}
	body, _ := json.Marshal(payload)

	s.mu.RLock()
	subs := make([]*types.ChangelogSubscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		if sub.Enabled {
			subs = append(subs, sub)
		}
	}
	s.mu.RUnlock()

	result.Subscribers = len(subs)
	for _, sub := range subs {
		if sub.BreakingOnly && len(breaking) == 0 {
			continue
		}
		if sub.WebhookURL != "" {
			if err := s.postWebhook(ctx, sub.WebhookURL, body); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("webhook %s: %v", sub.ID, err))
			} else {
				result.WebhooksSent++
			}
		}
		if sub.Email != "" && s.cfg.Email.Enabled {
			if err := s.sendEmail(sub.Email, version, body); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("email %s: %v", sub.ID, err))
			} else {
				result.EmailsSent++
			}
		}
	}
	return result
}

func (s *Service) postWebhook(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kranix-Event", string(types.WebhookEventChangelogBreaking))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) sendEmail(to, version string, body []byte) error {
	cfg := s.cfg.Email
	if cfg.SMTPHost == "" {
		log.Printf("changelog email to %s (SMTP not configured): %s", to, string(body))
		return nil
	}
	port := cfg.SMTPPort
	if port == "" {
		port = "587"
	}
	from := cfg.From
	if from == "" {
		from = "kranix-api@localhost"
	}
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: [Kranix API] Breaking changes in %s\r\nContent-Type: application/json\r\n\r\n%s",
		to, version, string(body)))
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, port)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
