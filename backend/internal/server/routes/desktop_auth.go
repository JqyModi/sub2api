package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterDesktopAuthRoutes exposes the narrow bridge used by the desktop app.
// It deliberately does not expose administrator credentials or management APIs.
func RegisterDesktopAuthRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtAuth middleware.JWTAuthMiddleware, settingService *service.SettingService, panelRateLimiter *middleware.PanelRateLimiter) {
	public := v1.Group("/desktop-auth")
	public.Use(middleware.BackendModeAuthGuard(settingService))
	{
		public.POST("/sessions", h.DesktopAuth.Start)
		public.POST("/token", h.DesktopAuth.PollToken)
		public.POST("/events", panelRateLimiter.PublicIP(), h.DesktopAuth.TrackEvent)
	}

	authorized := v1.Group("/desktop-auth/sessions")
	authorized.Use(gin.HandlerFunc(jwtAuth))
	authorized.Use(middleware.BackendModeUserGuard(settingService))
	{
		authorized.POST("/:sessionID/approve", h.DesktopAuth.Approve)
		authorized.POST("/:sessionID/cancel", h.DesktopAuth.Cancel)
	}
}
