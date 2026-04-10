package handlers

import (
	"net/http"
	"net/url"

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
	c.Redirect(http.StatusTemporaryRedirect, h.authService.BeginLogin())
}

func (h *AuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	user, token, err := h.authService.HandleCallback(c.Request.Context(), code, state)
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
	return h.cfg.FrontendBaseURL + path
}
