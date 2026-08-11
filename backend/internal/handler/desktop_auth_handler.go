package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopAuthHandler struct {
	service        *service.DesktopAuthService
	paymentService *service.PaymentService
}

func NewDesktopAuthHandler(desktopAuthService *service.DesktopAuthService, paymentService *service.PaymentService) *DesktopAuthHandler {
	return &DesktopAuthHandler{service: desktopAuthService, paymentService: paymentService}
}

type startDesktopAuthRequest struct {
	ClientID      string `json:"client_id" binding:"required"`
	CodeChallenge string `json:"code_challenge" binding:"required"`
	DeviceName    string `json:"device_name"`
	Platform      string `json:"platform"`
	AppVersion    string `json:"app_version"`
	CampaignID    string `json:"campaign_id"`
}

type desktopGrowthEventRequest struct {
	EventType  string `json:"event_type" binding:"required"`
	CampaignID string `json:"campaign_id"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type pollDesktopAuthRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	CodeVerifier string `json:"code_verifier" binding:"required"`
}

type approveDesktopAuthRequest struct {
	SubscriptionID *int64 `json:"subscription_id"`
}

func (h *DesktopAuthHandler) Start(c *gin.Context) {
	var req startDesktopAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid desktop authorization request")
		return
	}
	baseURL := requestOrigin(c)
	result, err := h.service.CreateSession(c.Request.Context(), req.ClientID, req.CodeChallenge, req.DeviceName, baseURL+"/desktop/authorize")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.recordGrowthEvent(c, service.GrowthEventInput{
		EventType:   service.GrowthEventDesktopAuthStarted,
		CampaignID:  req.CampaignID,
		Platform:    req.Platform,
		AppVersion:  req.AppVersion,
		SessionHash: hashDesktopSession(result.SessionID),
	})
	response.Success(c, result)
}

func (h *DesktopAuthHandler) TrackEvent(c *gin.Context) {
	var req desktopGrowthEventRequest
	if err := c.ShouldBindJSON(&req); err != nil || !service.IsPublicGrowthEventType(req.EventType) {
		response.BadRequest(c, "Invalid desktop growth event")
		return
	}
	if h.paymentService == nil {
		response.Success(c, gin.H{"ok": true})
		return
	}
	if err := h.paymentService.RecordGrowthEvent(c.Request.Context(), service.GrowthEventInput{
		EventType:  req.EventType,
		CampaignID: req.CampaignID,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *DesktopAuthHandler) PollToken(c *gin.Context) {
	var req pollDesktopAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid desktop authorization token request")
		return
	}
	session, status, err := h.service.PollToken(c.Request.Context(), req.SessionID, req.CodeVerifier)
	if err != nil {
		if errors.Is(err, service.ErrDesktopAuthSessionNotFound) || errors.Is(err, service.ErrDesktopAuthSessionExpired) {
			response.Error(c, http.StatusGone, "Desktop authorization session expired")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	if session.State != service.DesktopAuthAuthorized {
		statusCode := http.StatusConflict
		if session.State == service.DesktopAuthPaymentRequired {
			statusCode = http.StatusPaymentRequired
		}
		c.JSON(statusCode, gin.H{"code": statusCode, "message": "authorization is not complete", "data": status})
		return
	}
	userID := session.UserID
	h.recordGrowthEvent(c, service.GrowthEventInput{
		EventType:   service.GrowthEventTokenRedeemed,
		SessionHash: hashDesktopSession(session.ID),
		UserID:      &userID,
	})
	response.Success(c, gin.H{
		"access_token":    session.AccessToken,
		"base_url":        session.BaseURL,
		"default_model":   session.DefaultModel,
		"expires_at":      session.SubscriptionExpiresAt,
		"provider_name":   session.ProviderName,
		"subscription_id": session.SubscriptionID,
	})
}

func (h *DesktopAuthHandler) recordGrowthEvent(c *gin.Context, input service.GrowthEventInput) {
	if h.paymentService == nil {
		return
	}
	if err := h.paymentService.RecordGrowthEvent(c.Request.Context(), input); err != nil {
		slog.Warn("growth event recording failed", "event_type", input.EventType, "error", err)
	}
}

func hashDesktopSession(sessionID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	return hex.EncodeToString(digest[:])
}

func (h *DesktopAuthHandler) Approve(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req approveDesktopAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid desktop authorization approval request")
		return
	}
	result, err := h.service.ApproveSession(c.Request.Context(), strings.TrimSpace(c.Param("sessionID")), subject.UserID, req.SubscriptionID, requestOrigin(c))
	if err != nil {
		if errors.Is(err, service.ErrDesktopAuthInvalidSubscription) {
			response.BadRequest(c, "The selected subscription is not active")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DesktopAuthHandler) Cancel(c *gin.Context) {
	if err := h.service.CancelSession(c.Request.Context(), strings.TrimSpace(c.Param("sessionID"))); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func requestOrigin(c *gin.Context) string {
	scheme := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	if scheme != "http" && scheme != "https" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + c.Request.Host
}
