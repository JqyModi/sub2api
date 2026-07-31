package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopAuthHTTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newDesktopAuthHandlerStore()
	issuer := &desktopAuthHandlerKeyIssuer{key: &service.APIKey{ID: 1, Key: "sk-http-device-key"}}
	reader := &desktopAuthHandlerSubscriptionReader{subscriptions: []service.UserSubscription{{
		ID: 9, UserID: 42, GroupID: 7, ExpiresAt: time.Now().Add(24 * time.Hour),
	}}}
	h := NewDesktopAuthHandler(service.NewDesktopAuthService(store, issuer, reader))
	router := gin.New()
	router.POST("/api/v1/desktop-auth/sessions", h.Start)
	router.POST("/api/v1/desktop-auth/token", h.PollToken)
	router.POST("/api/v1/desktop-auth/sessions/:sessionID/approve", func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 42})
		h.Approve(c)
	})

	verifier := "handler-e2e-pkce-verifier"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	start := requestDesktopAuth(t, router, http.MethodPost, "/api/v1/desktop-auth/sessions", `{"client_id":"codex-multi-launcher","code_challenge":"`+challenge+`","device_name":"Test Mac"}`)
	require.Equal(t, http.StatusOK, start.Code)
	var created struct {
		Data struct {
			SessionID string `json:"session_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &created))
	require.NotEmpty(t, created.Data.SessionID)

	approve := requestDesktopAuth(t, router, http.MethodPost, "/api/v1/desktop-auth/sessions/"+created.Data.SessionID+"/approve", "{}")
	require.Equal(t, http.StatusOK, approve.Code)
	require.Equal(t, int64(7), *issuer.request.GroupID)

	token := requestDesktopAuth(t, router, http.MethodPost, "/api/v1/desktop-auth/token", `{"session_id":"`+created.Data.SessionID+`","code_verifier":"`+verifier+`"}`)
	require.Equal(t, http.StatusOK, token.Code)
	var exchanged struct {
		Data struct {
			AccessToken string `json:"access_token"`
			BaseURL     string `json:"base_url"`
			ExpiresAt   string `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(token.Body.Bytes(), &exchanged))
	require.Equal(t, "sk-http-device-key", exchanged.Data.AccessToken)
	require.Equal(t, "https://example.test/v1", exchanged.Data.BaseURL)
	require.NotEmpty(t, exchanged.Data.ExpiresAt)

	reused := requestDesktopAuth(t, router, http.MethodPost, "/api/v1/desktop-auth/token", `{"session_id":"`+created.Data.SessionID+`","code_verifier":"`+verifier+`"}`)
	require.NotEqual(t, http.StatusOK, reused.Code)
}

func requestDesktopAuth(t *testing.T, router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "example.test"
	req.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, req)
	return responseRecorder
}

type desktopAuthHandlerStore struct {
	values map[string]string
}

func newDesktopAuthHandlerStore() *desktopAuthHandlerStore {
	return &desktopAuthHandlerStore{values: map[string]string{}}
}

func (s *desktopAuthHandlerStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", service.ErrDesktopAuthStoreNotFound
	}
	return value, nil
}

func (s *desktopAuthHandlerStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.values[key] = value
	return nil
}

func (s *desktopAuthHandlerStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type desktopAuthHandlerKeyIssuer struct {
	key     *service.APIKey
	request service.CreateAPIKeyRequest
}

func (s *desktopAuthHandlerKeyIssuer) Create(_ context.Context, _ int64, request service.CreateAPIKeyRequest) (*service.APIKey, error) {
	s.request = request
	return s.key, nil
}

type desktopAuthHandlerSubscriptionReader struct {
	subscriptions []service.UserSubscription
}

func (s *desktopAuthHandlerSubscriptionReader) ListActiveUserSubscriptions(_ context.Context, _ int64) ([]service.UserSubscription, error) {
	return s.subscriptions, nil
}
