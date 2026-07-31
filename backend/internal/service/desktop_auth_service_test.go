package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDesktopAuthSessionUsesPKCEAndIsConsumedOnce(t *testing.T) {
	miniRedis := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	svc := NewDesktopAuthService(client, nil, nil)
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

func TestDesktopAuthSessionExpiresInRedis(t *testing.T) {
	miniRedis := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	svc := NewDesktopAuthService(client, nil, nil)

	session, err := svc.CreateSession(context.Background(), desktopAuthClientID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", "", "https://service.example/desktop/authorize")
	require.NoError(t, err)
	miniRedis.FastForward(desktopAuthSessionTTL + time.Second)
	_, err = svc.load(context.Background(), session.SessionID)
	require.ErrorIs(t, err, ErrDesktopAuthSessionNotFound)
}
