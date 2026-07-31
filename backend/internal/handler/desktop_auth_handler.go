package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopAuthHandler struct {
	service *service.DesktopAuthService
}

func NewDesktopAuthHandler(desktopAuthService *service.DesktopAuthService) *DesktopAuthHandler {
	return &DesktopAuthHandler{service: desktopAuthService}
}

type startDesktopAuthRequest struct {
	ClientID      string `json:"client_id" binding:"required"`
	CodeChallenge string `json:"code_challenge" binding:"required"`
	DeviceName    string `json:"device_name"`
}

type pollDesktopAuthRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	CodeVerifier string `json:"code_verifier" binding:"required"`
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
	response.Success(c, result)
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
	response.Success(c, gin.H{
		"access_token":  session.AccessToken,
		"base_url":      session.BaseURL,
		"default_model": session.DefaultModel,
		"expires_at":    session.SubscriptionExpiresAt,
		"provider_name": session.ProviderName,
	})
}

func (h *DesktopAuthHandler) Approve(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.ApproveSession(c.Request.Context(), strings.TrimSpace(c.Param("sessionID")), subject.UserID, requestOrigin(c))
	if err != nil {
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
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
