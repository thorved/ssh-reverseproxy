package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPListenAddr  string
	FrontendBaseURL string
	DatabasePath    string

	SessionCookieName string
	SessionTTL        time.Duration
	SessionSecure     bool
	SessionSecret     string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       []string

	AdminEmails []string

	SSHListenAddr        string
	SSHHostKeyPath       string
	SSHKnownHostsPath    string
	SSHServerIdent       string
	SSHDialTimeout       time.Duration
	SSHAcceptUnknownHost bool
}

func MustLoad() Config {
	httpAddr := envOr("HTTP_LISTEN_ADDR", "")
	if httpAddr == "" {
		httpAddr = ":" + envOr("PORT", "8080")
	}

	frontendBaseURL := strings.TrimRight(envOr("FRONTEND_BASE_URL", "http://localhost:3000"), "/")
	redirectURL := strings.TrimSpace(os.Getenv("OIDC_REDIRECT_URL"))
	if redirectURL == "" {
		redirectURL = frontendBaseURL + "/api/auth/oidc/callback"
	}

	return Config{
		Env:                  envOr("ENV", "development"),
		HTTPListenAddr:       httpAddr,
		FrontendBaseURL:      frontendBaseURL,
		DatabasePath:         envOr("DATABASE_PATH", "./data/ssh-reverseproxy.db"),
		SessionCookieName:    envOr("SESSION_COOKIE_NAME", "sshrp_session"),
		SessionTTL:           envDurationOr("SESSION_TTL", 7*24*time.Hour),
		SessionSecure:        envBoolOr("SESSION_SECURE", false),
		SessionSecret:        envOr("SESSION_SECRET", "development-session-secret-change-me"),
		OIDCIssuerURL:        strings.TrimRight(os.Getenv("OIDC_ISSUER_URL"), "/"),
		OIDCClientID:         strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID")),
		OIDCClientSecret:     strings.TrimSpace(os.Getenv("OIDC_CLIENT_SECRET")),
		OIDCRedirectURL:      redirectURL,
		OIDCScopes:           envCSVOr("OIDC_SCOPES", []string{"openid", "profile", "email"}),
		AdminEmails:          normalizeEmails(envCSVOr("ADMIN_EMAILS", nil)),
		SSHListenAddr:        sshListenAddr(),
		SSHHostKeyPath:       strings.TrimSpace(os.Getenv("SSH_HOST_KEY_PATH")),
		SSHKnownHostsPath:    strings.TrimSpace(os.Getenv("SSH_KNOWN_HOSTS")),
		SSHServerIdent:       envOr("SSH_SERVER_IDENT", "SSH-2.0-ssh-reverseproxy"),
		SSHDialTimeout:       envDurationOr("SSH_DIAL_TIMEOUT", 15*time.Second),
		SSHAcceptUnknownHost: envBoolOr("SSH_ACCEPT_UNKNOWN_UPSTREAM", false),
	}
}

func sshListenAddr() string {
	if v := strings.TrimSpace(os.Getenv("SSH_LISTEN_ADDR")); v != "" {
		return v
	}
	return ":" + envOr("SSH_PORT", "2222")
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func envCSVOr(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}

func normalizeEmails(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		email := strings.ToLower(strings.TrimSpace(item))
		if email != "" {
			out = append(out, email)
		}
	}
	return out
}

func (c Config) HTTPPort() int {
	if idx := strings.LastIndex(c.HTTPListenAddr, ":"); idx >= 0 && idx < len(c.HTTPListenAddr)-1 {
		port, err := strconv.Atoi(c.HTTPListenAddr[idx+1:])
		if err == nil {
			return port
		}
	}
	return 8080
}

func (c Config) SSHPort() int {
	if idx := strings.LastIndex(c.SSHListenAddr, ":"); idx >= 0 && idx < len(c.SSHListenAddr)-1 {
		port, err := strconv.Atoi(c.SSHListenAddr[idx+1:])
		if err == nil {
			return port
		}
	}
	return 2222
}
