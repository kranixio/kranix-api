package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kranix-io/kranix-packages/logging"
	"github.com/kranix-io/kranix-packages/types"
)

// Service manages webhook delivery.
type Service struct {
	webhooks      map[string]*types.Webhook
	deliveries    map[string]*types.WebhookDelivery
	mu            sync.RWMutex
	logger        *logging.Logger
	httpClient    *http.Client
	retryQueue    chan *types.WebhookDelivery
	maxRetries    int
	retryInterval time.Duration
}

// Config represents webhook service configuration.
type Config struct {
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
	Timeout       time.Duration `yaml:"timeout"`
}

// NewService creates a new webhook service.
func NewService(config Config, logger *logging.Logger) *Service {
	s := &Service{
		webhooks:   make(map[string]*types.Webhook),
		deliveries: make(map[string]*types.WebhookDelivery),
		logger:     logger,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		retryQueue:    make(chan *types.WebhookDelivery, 1000),
		maxRetries:    config.MaxRetries,
		retryInterval: config.RetryInterval,
	}

	// Start retry worker
	go s.retryWorker()

	return s
}

// RegisterWebhook registers a new webhook.
func (s *Service) RegisterWebhook(webhook *types.Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	webhook.ID = generateID()
	webhook.CreatedAt = time.Now()
	webhook.UpdatedAt = time.Now()
	s.webhooks[webhook.ID] = webhook

	s.logger.Info("webhook registered", "id", webhook.ID, "name", webhook.Name, "url", webhook.URL)
	return nil
}

// UnregisterWebhook removes a webhook.
func (s *Service) UnregisterWebhook(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.webhooks, id)
	s.logger.Info("webhook unregistered", "id", id)
	return nil
}

// GetWebhook retrieves a webhook by ID.
func (s *Service) GetWebhook(id string) (*types.Webhook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	webhook, ok := s.webhooks[id]
	if !ok {
		return nil, fmt.Errorf("webhook not found")
	}
	return webhook, nil
}

// ListWebhooks lists all webhooks.
func (s *Service) ListWebhooks() []*types.Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	webhooks := make([]*types.Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		webhooks = append(webhooks, webhook)
	}
	return webhooks
}

// Trigger sends a webhook event to all matching webhooks.
func (s *Service) Trigger(ctx context.Context, eventType types.WebhookEvent, payload *types.WebhookPayload) error {
	s.mu.RLock()
	webhooks := make([]*types.Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		if !webhook.Enabled {
			continue
		}
		if s.shouldTrigger(webhook, eventType) {
			webhooks = append(webhooks, webhook)
		}
	}
	s.mu.RUnlock()

	if len(webhooks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(webhooks))

	for _, webhook := range webhooks {
		wg.Add(1)
		go func(wh *types.Webhook) {
			defer wg.Done()
			if err := s.deliver(ctx, wh, eventType, payload); err != nil {
				errChan <- err
			}
		}(webhook)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to deliver some webhooks: %v", errors)
	}

	return nil
}

// shouldTrigger checks if a webhook should be triggered for an event type.
func (s *Service) shouldTrigger(webhook *types.Webhook, eventType types.WebhookEvent) bool {
	for _, event := range webhook.Events {
		if event == eventType {
			return true
		}
	}
	return false
}

// deliver sends a webhook to a specific endpoint.
func (s *Service) deliver(ctx context.Context, webhook *types.Webhook, eventType types.WebhookEvent, payload *types.WebhookPayload) error {
	delivery := &types.WebhookDelivery{
		ID:          generateID(),
		WebhookID:   webhook.ID,
		EventType:   eventType,
		Payload:     *payload,
		Attempt:     1,
		MaxAttempts: s.maxRetries,
		DeliveredAt: time.Now(),
	}

	body, err := s.buildPayload(webhook, eventType, payload)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kranix-Event", string(eventType))
	req.Header.Set("X-Kranix-Delivery-ID", delivery.ID)
	req.Header.Set("X-Kranix-Timestamp", time.Now().Format(time.RFC3339))

	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Add signature if secret is provided
	if webhook.Secret != "" {
		signature := s.signPayload(body, webhook.Secret)
		req.Header.Set("X-Kranix-Signature", signature)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		delivery.Success = false
		delivery.StatusCode = 0
		delivery.Response = err.Error()
		s.recordDelivery(delivery)

		// Queue for retry
		if delivery.Attempt < s.maxRetries {
			delivery.NextRetryAt = time.Now().Add(s.retryInterval)
			s.retryQueue <- delivery
		}
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	delivery.StatusCode = resp.StatusCode
	delivery.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	respBody, _ := json.Marshal(map[string]interface{}{
		"status": resp.StatusCode,
		"body":   resp.Body,
	})
	delivery.Response = string(respBody)

	s.recordDelivery(delivery)

	if !delivery.Success {
		if delivery.Attempt < s.maxRetries {
			delivery.NextRetryAt = time.Now().Add(s.retryInterval)
			s.retryQueue <- delivery
		}
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	s.logger.Info("webhook delivered successfully",
		"webhook_id", webhook.ID,
		"delivery_id", delivery.ID,
		"event_type", eventType,
		"status_code", resp.StatusCode)

	return nil
}

// buildPayload builds the payload for a webhook.
func (s *Service) buildPayload(webhook *types.Webhook, eventType types.WebhookEvent, payload *types.WebhookPayload) ([]byte, error) {
	// For provider-specific formatting, we could add logic here
	// For now, return the standard payload
	return json.Marshal(payload)
}

// signPayload signs the payload with HMAC-SHA256.
func (s *Service) signPayload(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// recordDelivery records a webhook delivery.
func (s *Service) recordDelivery(delivery *types.WebhookDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries[delivery.ID] = delivery
}

// retryWorker processes failed webhook deliveries for retry.
func (s *Service) retryWorker() {
	for delivery := range s.retryQueue {
		time.Sleep(time.Until(delivery.NextRetryAt))

		s.mu.RLock()
		webhook, ok := s.webhooks[delivery.WebhookID]
		s.mu.RUnlock()

		if !ok || !webhook.Enabled {
			continue
		}

		delivery.Attempt++
		if err := s.deliver(context.Background(), webhook, delivery.EventType, &delivery.Payload); err != nil {
			s.logger.Error("webhook retry failed",
				"delivery_id", delivery.ID,
				"attempt", delivery.Attempt,
				"error", err)
		}
	}
}

// GetDelivery retrieves a delivery by ID.
func (s *Service) GetDelivery(id string) (*types.WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	delivery, ok := s.deliveries[id]
	if !ok {
		return nil, fmt.Errorf("delivery not found")
	}
	return delivery, nil
}

// ListDeliveries lists all deliveries for a webhook.
func (s *Service) ListDeliveries(webhookID string) []*types.WebhookDelivery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deliveries := make([]*types.WebhookDelivery, 0)
	for _, delivery := range s.deliveries {
		if delivery.WebhookID == webhookID {
			deliveries = append(deliveries, delivery)
		}
	}
	return deliveries
}

// generateID generates a unique ID.
func generateID() string {
	return fmt.Sprintf("wh_%d", time.Now().UnixNano())
}

// FormatPayloadForSlack formats a payload for Slack.
func FormatPayloadForSlack(payload *types.WebhookPayload, config *types.SlackConfig) map[string]interface{} {
	color := "#36a64f" // green for success
	if payload.Event == types.WebhookEventDeployFailure ||
		payload.Event == types.WebhookEventHealthCheckFail {
		color = "#dc3545" // red for failures
	}

	return map[string]interface{}{
		"channel":    config.Channel,
		"username":   config.Username,
		"icon_emoji": config.IconEmoji,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("Kranix Event: %s", payload.Event),
				"text":  fmt.Sprintf("Workload: %s\nNamespace: %s", payload.WorkloadID, payload.Namespace),
				"ts":    payload.Timestamp.Unix(),
			},
		},
	}
}

// FormatPayloadForPagerDuty formats a payload for PagerDuty.
func FormatPayloadForPagerDuty(payload *types.WebhookPayload, config *types.PagerDutyConfig) map[string]interface{} {
	severity := "info"
	if payload.Event == types.WebhookEventDeployFailure ||
		payload.Event == types.WebhookEventHealthCheckFail {
		severity = config.Severity
		if severity == "" {
			severity = "critical"
		}
	}

	return map[string]interface{}{
		"routing_key":  config.RoutingKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":   fmt.Sprintf("Kranix Event: %s", payload.Event),
			"severity":  severity,
			"source":    "kranix",
			"timestamp": payload.Timestamp.Format(time.RFC3339),
			"custom_details": map[string]interface{}{
				"workload_id": payload.WorkloadID,
				"namespace":   payload.Namespace,
				"event":       payload.Event,
				"data":        payload.Data,
			},
		},
	}
}
