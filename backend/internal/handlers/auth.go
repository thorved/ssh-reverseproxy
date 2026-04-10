package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/auth"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/middleware"
)

type AuthHandler struct {
	cfg         config.Config
	authService *auth.Service
}

func NewAuthHandler(cfg config.Config, authService *auth.Service) *AuthHandler {
	return &AuthHandler{cfg: cfg, authService: authService}
}

func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, h.authService.BeginLogin(h.callbackURL(c)))
}

func (h *AuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	user, token, err := h.authService.HandleCallback(
		c.Request.Context(),
		code,
		state,
		h.callbackURL(c),
	)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.redirectURL("/login?error="+url.QueryEscape(err.Error())))
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
	})

	target := "/dashboard"
	if user.Role == "admin" {
		target = "/admin/users"
	}
	c.Redirect(http.StatusTemporaryRedirect, h.redirectURL(target))
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.JSON(http.StatusOK, user.ToAuthResponse())
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(h.cfg.SessionCookieName)
	_ = h.authService.DeleteSession(c.Request.Context(), token)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) redirectURL(path string) string {
	baseURL := h.publicBaseURL()
	if h.cfg.Env == "production" {
		return path
	}
	if baseURL == "" {
		return path
	}
	return strings.TrimRight(baseURL, "/") + path
}

func (h *AuthHandler) callbackURL(c *gin.Context) string {
	baseURL := h.publicBaseURLFromRequest(c)
	if baseURL == "" {
		baseURL = h.publicBaseURL()
	}
	if baseURL == "" {
		baseURL = strings.TrimRight(h.cfg.FrontendBaseURL, "/")
	}
	return strings.TrimRight(baseURL, "/") + "/api/auth/oidc/callback"
}

func (h *AuthHandler) publicBaseURLFromRequest(c *gin.Context) string {
	if forwardedHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
		if scheme == "" {
			scheme = "http"
		}
		return scheme + "://" + forwardedHost
	}

	if referer := strings.TrimSpace(c.GetHeader("Referer")); referer != "" {
		if parsed, err := url.Parse(referer); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}

	if c.Request != nil && c.Request.Host != "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + c.Request.Host
	}

	return ""
}

func (h *AuthHandler) publicBaseURL() string {
	return strings.TrimRight(h.cfg.FrontendBaseURL, "/")
}
