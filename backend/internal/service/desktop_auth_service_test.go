package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

	session, err := svc.load(context.Background(), created.SessionID)
	require.NoError(t, err)
	session.State = DesktopAuthAuthorized
	session.AccessToken = "sk-test-device-key"
	session.BaseURL = "https://service.example/v1"
	session.DefaultModel = desktopAuthDefaultModel
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
