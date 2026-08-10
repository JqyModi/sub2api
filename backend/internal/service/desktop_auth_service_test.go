package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDesktopAuthSessionUsesPKCEAndIsConsumedOnce(t *testing.T) {
	store := newTestDesktopAuthStore()
	svc := NewDesktopAuthService(store, nil, nil)
	verifier := "desktop-auth-verifier-for-test"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	created, err := svc.CreateSession(context.Background(), desktopAuthClientID, challenge, "Test device", "https://service.example/desktop/authorize")
	require.NoError(t, err)
	require.Equal(t, "https://service.example/desktop/authorize?session="+created.SessionID, created.AuthorizationURL)
	require.Equal(t, 30*60, created.ExpiresIn)

	session, err := svc.load(context.Background(), created.SessionID)
	require.NoError(t, err)
	session.State = DesktopAuthAuthorized
	session.AccessToken = "sk-test-device-key"
	session.BaseURL = "https://service.example/v1"
	session.DefaultModel = desktopAuthDefaultModel()
	session.ProviderName = "Test service"
	require.NoError(t, svc.save(context.Background(), session))

	_, _, err = svc.PollToken(context.Background(), created.SessionID, "wrong-verifier")
	require.ErrorIs(t, err, ErrDesktopAuthInvalidVerifier)

	consumed, status, err := svc.PollToken(context.Background(), created.SessionID, verifier)
	require.NoError(t, err)
	require.Equal(t, DesktopAuthAuthorized, status.State)
	require.Equal(t, "sk-test-device-key", consumed.AccessToken)

	_, _, err = svc.PollToken(context.Background(), created.SessionID, verifier)
	require.ErrorIs(t, err, ErrDesktopAuthSessionNotFound)
}

func TestDesktopAuthSessionExpiresInStore(t *testing.T) {
	store := newTestDesktopAuthStore()
	svc := NewDesktopAuthService(store, nil, nil)

	session, err := svc.CreateSession(context.Background(), desktopAuthClientID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", "", "https://service.example/desktop/authorize")
	require.NoError(t, err)
	store.expiredAt[desktopAuthSessionPrefix+session.SessionID] = time.Now().Add(-time.Second)
	_, err = svc.load(context.Background(), session.SessionID)
	require.ErrorIs(t, err, ErrDesktopAuthSessionNotFound)
}

func TestDesktopAuthApprovalCreatesUserDeviceKey(t *testing.T) {
	t.Setenv("DESKTOP_AUTH_DEFAULT_MODEL", "gpt-5.6-sol")
	store := newTestDesktopAuthStore()
	issuer := &testDesktopAuthKeyIssuer{key: &APIKey{ID: 99, Key: "sk-device-test"}}
	reader := &testDesktopAuthSubscriptionReader{subscriptions: []UserSubscription{{
		ID: 7, UserID: 42, GroupID: 12, ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}}}
	svc := NewDesktopAuthService(store, issuer, reader)
	verifier := "desktop-auth-approval-verifier"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	created, err := svc.CreateSession(context.Background(), desktopAuthClientID, challenge, "MacBook", "https://service.example/desktop/authorize")
	require.NoError(t, err)

	status, err := svc.ApproveSession(context.Background(), created.SessionID, 42, nil, "https://service.example")
	require.NoError(t, err)
	require.Equal(t, DesktopAuthAuthorized, status.State)
	require.Equal(t, int64(42), issuer.userID)
	require.Equal(t, int64(12), *issuer.request.GroupID)
	require.Equal(t, "Codex Multi Launcher - MacBook", issuer.request.Name)
	require.Nil(t, issuer.request.ExpiresAt)

	consumed, _, err := svc.PollToken(context.Background(), created.SessionID, verifier)
	require.NoError(t, err)
	require.Equal(t, "sk-device-test", consumed.AccessToken)
	require.Equal(t, "https://service.example/v1", consumed.BaseURL)
	require.Equal(t, "gpt-5.6-sol", consumed.DefaultModel)
	require.NotNil(t, consumed.SubscriptionExpiresAt)
	require.Equal(t, int64(7), consumed.SubscriptionID)
}

func TestDesktopAuthApprovalRequiresSelectionForMultipleSubscriptions(t *testing.T) {
	store := newTestDesktopAuthStore()
	issuer := &testDesktopAuthKeyIssuer{key: &APIKey{ID: 100, Key: "sk-selected-device"}}
	reader := &testDesktopAuthSubscriptionReader{subscriptions: []UserSubscription{
		{ID: 8, UserID: 42, GroupID: 20, ExpiresAt: time.Now().Add(30 * 24 * time.Hour), Group: &Group{Name: "Codex Pro"}},
		{ID: 7, UserID: 42, GroupID: 10, ExpiresAt: time.Now().Add(15 * 24 * time.Hour), Group: &Group{Name: "Codex Lite"}},
	}}
	svc := NewDesktopAuthService(store, issuer, reader)
	created, err := svc.CreateSession(context.Background(), desktopAuthClientID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", "Windows", "https://service.example/desktop/authorize")
	require.NoError(t, err)

	status, err := svc.ApproveSession(context.Background(), created.SessionID, 42, nil, "https://service.example")
	require.NoError(t, err)
	require.Equal(t, DesktopAuthSelectionRequired, status.State)
	require.Len(t, status.Subscriptions, 2)
	require.Equal(t, "Codex Pro", status.Subscriptions[0].GroupName)
	require.Zero(t, issuer.userID)

	selectedID := int64(7)
	status, err = svc.ApproveSession(context.Background(), created.SessionID, 42, &selectedID, "https://service.example")
	require.NoError(t, err)
	require.Equal(t, DesktopAuthAuthorized, status.State)
	require.Equal(t, int64(10), *issuer.request.GroupID)
	require.Nil(t, issuer.request.ExpiresAt)
}

func TestDesktopAuthApprovalRejectsInactiveSelection(t *testing.T) {
	store := newTestDesktopAuthStore()
	issuer := &testDesktopAuthKeyIssuer{key: &APIKey{ID: 100, Key: "sk-selected-device"}}
	reader := &testDesktopAuthSubscriptionReader{subscriptions: []UserSubscription{
		{ID: 8, UserID: 42, GroupID: 20, ExpiresAt: time.Now().Add(30 * 24 * time.Hour)},
		{ID: 7, UserID: 42, GroupID: 10, ExpiresAt: time.Now().Add(15 * 24 * time.Hour)},
	}}
	svc := NewDesktopAuthService(store, issuer, reader)
	created, err := svc.CreateSession(context.Background(), desktopAuthClientID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", "Windows", "https://service.example/desktop/authorize")
	require.NoError(t, err)

	selectedID := int64(999)
	_, err = svc.ApproveSession(context.Background(), created.SessionID, 42, &selectedID, "https://service.example")
	require.ErrorIs(t, err, ErrDesktopAuthInvalidSubscription)
	require.Zero(t, issuer.userID)
}

func TestDesktopAuthDefaultModelFallback(t *testing.T) {
	require.NoError(t, os.Unsetenv("DESKTOP_AUTH_DEFAULT_MODEL"))
	require.Equal(t, "gpt-5.5", desktopAuthDefaultModel())
}

type testDesktopAuthStore struct {
	values    map[string]string
	expiredAt map[string]time.Time
}

func newTestDesktopAuthStore() *testDesktopAuthStore {
	return &testDesktopAuthStore{values: map[string]string{}, expiredAt: map[string]time.Time{}}
}

func (s *testDesktopAuthStore) Get(_ context.Context, key string) (string, error) {
	if expiresAt, ok := s.expiredAt[key]; ok && time.Now().After(expiresAt) {
		delete(s.values, key)
		return "", ErrDesktopAuthStoreNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrDesktopAuthStoreNotFound
	}
	return value, nil
}

func (s *testDesktopAuthStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.values[key] = value
	s.expiredAt[key] = time.Now().Add(ttl)
	return nil
}

func (s *testDesktopAuthStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	delete(s.expiredAt, key)
	return nil
}

type testDesktopAuthKeyIssuer struct {
	key     *APIKey
	request CreateAPIKeyRequest
	userID  int64
}

func (s *testDesktopAuthKeyIssuer) Create(_ context.Context, userID int64, request CreateAPIKeyRequest) (*APIKey, error) {
	s.userID = userID
	s.request = request
	return s.key, nil
}

type testDesktopAuthSubscriptionReader struct {
	subscriptions []UserSubscription
}

func (s *testDesktopAuthSubscriptionReader) ListActiveUserSubscriptions(_ context.Context, _ int64) ([]UserSubscription, error) {
	return s.subscriptions, nil
}
