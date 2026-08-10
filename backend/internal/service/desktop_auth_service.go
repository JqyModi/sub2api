package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	desktopAuthClientID      = "codex-multi-launcher"
	desktopAuthSessionTTL    = 30 * time.Minute
	desktopAuthPollInterval  = 2
	desktopAuthSessionPrefix = "desktop-auth:session:"
	desktopAuthFallbackModel = "gpt-5.5"
	desktopAuthKeyNamePrefix = "Codex Multi Launcher"
)

var (
	ErrDesktopAuthSessionNotFound     = errors.New("desktop authorization session not found")
	ErrDesktopAuthStoreNotFound       = errors.New("desktop authorization store entry not found")
	ErrDesktopAuthSessionExpired      = errors.New("desktop authorization session expired")
	ErrDesktopAuthInvalidClient       = errors.New("desktop authorization client is invalid")
	ErrDesktopAuthInvalidVerifier     = errors.New("desktop authorization verifier is invalid")
	ErrDesktopAuthInvalidSubscription = errors.New("desktop authorization subscription is invalid")
	ErrDesktopAuthNotAuthorized       = errors.New("desktop authorization is not complete")
)

type DesktopAuthSessionState string

const (
	DesktopAuthPending           DesktopAuthSessionState = "pending"
	DesktopAuthPaymentRequired   DesktopAuthSessionState = "payment_required"
	DesktopAuthSelectionRequired DesktopAuthSessionState = "selection_required"
	DesktopAuthAuthorized        DesktopAuthSessionState = "authorized"
	DesktopAuthDenied            DesktopAuthSessionState = "denied"
	DesktopAuthExpired           DesktopAuthSessionState = "expired"
)

type DesktopAuthSession struct {
	ID                    string                          `json:"id"`
	ClientID              string                          `json:"client_id"`
	CodeChallenge         string                          `json:"code_challenge"`
	DeviceName            string                          `json:"device_name"`
	State                 DesktopAuthSessionState         `json:"state"`
	UserID                int64                           `json:"user_id,omitempty"`
	APIKeyID              int64                           `json:"api_key_id,omitempty"`
	SubscriptionID        int64                           `json:"subscription_id,omitempty"`
	SubscriptionOptions   []DesktopAuthSubscriptionOption `json:"subscription_options,omitempty"`
	AccessToken           string                          `json:"access_token,omitempty"`
	BaseURL               string                          `json:"base_url,omitempty"`
	DefaultModel          string                          `json:"default_model,omitempty"`
	ProviderName          string                          `json:"provider_name,omitempty"`
	SubscriptionExpiresAt *time.Time                      `json:"subscription_expires_at,omitempty"`
	ExpiresAt             time.Time                       `json:"expires_at"`
}

type DesktopAuthSessionResponse struct {
	SessionID        string `json:"session_id"`
	AuthorizationURL string `json:"authorization_url"`
	ExpiresIn        int    `json:"expires_in"`
	PollInterval     int    `json:"poll_interval"`
}

type DesktopAuthStatusResponse struct {
	State                 DesktopAuthSessionState         `json:"state"`
	ProviderName          string                          `json:"provider_name,omitempty"`
	DefaultModel          string                          `json:"default_model,omitempty"`
	SubscriptionExpiresAt *time.Time                      `json:"subscription_expires_at,omitempty"`
	Subscriptions         []DesktopAuthSubscriptionOption `json:"subscriptions,omitempty"`
}

type DesktopAuthSubscriptionOption struct {
	ID        int64     `json:"id"`
	GroupID   int64     `json:"group_id"`
	GroupName string    `json:"group_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DesktopAuthService struct {
	store              DesktopAuthStore
	apiKeyIssuer       DesktopAuthKeyIssuer
	subscriptionReader DesktopAuthSubscriptionReader
}

type DesktopAuthStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type DesktopAuthKeyIssuer interface {
	Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error)
}

type DesktopAuthSubscriptionReader interface {
	ListActiveUserSubscriptions(ctx context.Context, userID int64) ([]UserSubscription, error)
}

func NewDesktopAuthService(store DesktopAuthStore, apiKeyIssuer DesktopAuthKeyIssuer, subscriptionReader DesktopAuthSubscriptionReader) *DesktopAuthService {
	return &DesktopAuthService{
		store:              store,
		apiKeyIssuer:       apiKeyIssuer,
		subscriptionReader: subscriptionReader,
	}
}

func (s *DesktopAuthService) CreateSession(ctx context.Context, clientID, codeChallenge, deviceName, authorizationURL string) (*DesktopAuthSessionResponse, error) {
	if clientID != desktopAuthClientID {
		return nil, ErrDesktopAuthInvalidClient
	}
	if !isValidCodeChallenge(codeChallenge) {
		return nil, ErrDesktopAuthInvalidVerifier
	}
	if s.store == nil {
		return nil, errors.New("desktop authorization storage is unavailable")
	}

	id, err := randomDesktopAuthID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := DesktopAuthSession{
		ID:            id,
		ClientID:      clientID,
		CodeChallenge: codeChallenge,
		DeviceName:    normalizeDesktopDeviceName(deviceName),
		State:         DesktopAuthPending,
		ExpiresAt:     now.Add(desktopAuthSessionTTL),
	}
	if err := s.save(ctx, session); err != nil {
		return nil, err
	}

	return &DesktopAuthSessionResponse{
		SessionID:        id,
		AuthorizationURL: authorizationURL + "?session=" + id,
		ExpiresIn:        int(desktopAuthSessionTTL / time.Second),
		PollInterval:     desktopAuthPollInterval,
	}, nil
}

func (s *DesktopAuthService) ApproveSession(ctx context.Context, sessionID string, userID int64, selectedSubscriptionID *int64, baseURL string) (*DesktopAuthStatusResponse, error) {
	session, err := s.load(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrDesktopAuthSessionExpired
	}
	if session.UserID != 0 && session.UserID != userID {
		return nil, ErrDesktopAuthSessionNotFound
	}
	if session.State == DesktopAuthAuthorized {
		return statusFromDesktopSession(session), nil
	}
	if s.subscriptionReader == nil || s.apiKeyIssuer == nil {
		return nil, errors.New("desktop authorization dependencies are unavailable")
	}

	subscriptions, err := s.subscriptionReader.ListActiveUserSubscriptions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	session.UserID = userID
	if len(subscriptions) == 0 {
		session.State = DesktopAuthPaymentRequired
		if err := s.save(ctx, session); err != nil {
			return nil, err
		}
		return statusFromDesktopSession(session), nil
	}

	if len(subscriptions) > 1 && selectedSubscriptionID == nil {
		session.State = DesktopAuthSelectionRequired
		session.SubscriptionOptions = desktopAuthSubscriptionOptions(subscriptions)
		if err := s.save(ctx, session); err != nil {
			return nil, err
		}
		return statusFromDesktopSession(session), nil
	}

	subscription := subscriptions[0]
	if selectedSubscriptionID != nil {
		matched := false
		for i := range subscriptions {
			if subscriptions[i].ID == *selectedSubscriptionID {
				subscription = subscriptions[i]
				matched = true
				break
			}
		}
		if !matched {
			return nil, ErrDesktopAuthInvalidSubscription
		}
	}
	groupID := subscription.GroupID
	subscriptionExpiresAt := subscription.ExpiresAt.UTC()
	key, err := s.apiKeyIssuer.Create(ctx, userID, CreateAPIKeyRequest{
		Name:    desktopAuthKeyName(session.DeviceName),
		GroupID: &groupID,
	})
	if err != nil {
		return nil, fmt.Errorf("create desktop API key: %w", err)
	}

	session.State = DesktopAuthAuthorized
	session.APIKeyID = key.ID
	session.SubscriptionID = subscription.ID
	session.AccessToken = key.Key
	session.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
	session.DefaultModel = desktopAuthDefaultModel()
	session.ProviderName = "Sub2API subscription"
	session.SubscriptionExpiresAt = &subscriptionExpiresAt
	if err := s.save(ctx, session); err != nil {
		return nil, err
	}
	return statusFromDesktopSession(session), nil
}

func desktopAuthSubscriptionOptions(subscriptions []UserSubscription) []DesktopAuthSubscriptionOption {
	options := make([]DesktopAuthSubscriptionOption, 0, len(subscriptions))
	for i := range subscriptions {
		subscription := subscriptions[i]
		groupName := fmt.Sprintf("Subscription %d", subscription.GroupID)
		if subscription.Group != nil && strings.TrimSpace(subscription.Group.Name) != "" {
			groupName = subscription.Group.Name
		}
		options = append(options, DesktopAuthSubscriptionOption{
			ID:        subscription.ID,
			GroupID:   subscription.GroupID,
			GroupName: groupName,
			ExpiresAt: subscription.ExpiresAt.UTC(),
		})
	}
	return options
}

func desktopAuthDefaultModel() string {
	if model := strings.TrimSpace(os.Getenv("DESKTOP_AUTH_DEFAULT_MODEL")); model != "" {
		return model
	}
	return desktopAuthFallbackModel
}

func (s *DesktopAuthService) PollToken(ctx context.Context, sessionID, codeVerifier string) (*DesktopAuthSession, *DesktopAuthStatusResponse, error) {
	session, err := s.load(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if session.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrDesktopAuthSessionExpired
	}
	if session.State != DesktopAuthAuthorized {
		return &session, statusFromDesktopSession(session), nil
	}
	if !verifyCodeChallenge(codeVerifier, session.CodeChallenge) {
		return nil, nil, ErrDesktopAuthInvalidVerifier
	}
	if session.AccessToken == "" || session.BaseURL == "" {
		return nil, nil, ErrDesktopAuthNotAuthorized
	}
	if err := s.store.Delete(ctx, desktopAuthSessionPrefix+session.ID); err != nil {
		return nil, nil, fmt.Errorf("consume desktop authorization session: %w", err)
	}
	return &session, statusFromDesktopSession(session), nil
}

func (s *DesktopAuthService) CancelSession(ctx context.Context, sessionID string) error {
	if s.store == nil {
		return nil
	}
	return s.store.Delete(ctx, desktopAuthSessionPrefix+sessionID)
}

func (s *DesktopAuthService) load(ctx context.Context, sessionID string) (DesktopAuthSession, error) {
	if s.store == nil {
		return DesktopAuthSession{}, errors.New("desktop authorization storage is unavailable")
	}
	raw, err := s.store.Get(ctx, desktopAuthSessionPrefix+sessionID)
	if errors.Is(err, ErrDesktopAuthStoreNotFound) {
		return DesktopAuthSession{}, ErrDesktopAuthSessionNotFound
	}
	if err != nil {
		return DesktopAuthSession{}, err
	}
	var session DesktopAuthSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return DesktopAuthSession{}, fmt.Errorf("decode desktop authorization session: %w", err)
	}
	return session, nil
}

func (s *DesktopAuthService) save(ctx context.Context, session DesktopAuthSession) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.store.Set(ctx, desktopAuthSessionPrefix+session.ID, string(raw), time.Until(session.ExpiresAt))
}

func statusFromDesktopSession(session DesktopAuthSession) *DesktopAuthStatusResponse {
	return &DesktopAuthStatusResponse{
		State:                 session.State,
		ProviderName:          session.ProviderName,
		DefaultModel:          session.DefaultModel,
		SubscriptionExpiresAt: session.SubscriptionExpiresAt,
		Subscriptions:         session.SubscriptionOptions,
	}
}

func verifyCodeChallenge(verifier, expected string) bool {
	sum := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func isValidCodeChallenge(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isUpper || isLower || isDigit || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func randomDesktopAuthID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "dsa_" + hex.EncodeToString(b), nil
}

func normalizeDesktopDeviceName(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "Codex Multi Launcher device"
	}
	if len(value) > 100 {
		return value[:100]
	}
	return value
}

func desktopAuthKeyName(deviceName string) string {
	return desktopAuthKeyNamePrefix + " - " + normalizeDesktopDeviceName(deviceName)
}
