package config

import (
	"os"
	"strings"
	"time"
)

// ServerConfig holds runtime configuration for the proxy server.
type ServerConfig struct {
	ListenAddr            string
	HostKeyPath           string
	KnownHosts            string
	ServerIdent           string
	DialTimeout           time.Duration
	AcceptUnknownUpstream bool

	// DB-only mapping source
	DBDSN             string
	DBTable           string
	DBTimeout         time.Duration
	DBRefreshInterval time.Duration
}

// MustLoad loads configuration from environment variables and applies defaults.
func MustLoad() ServerConfig {
	addr := getenvDefault("SSH_LISTEN_ADDR", "")
	port := getenvDefault("SSH_PORT", "")
	if addr == "" {
		if port == "" {
			addr = ":2222"
		} else {
			addr = ":" + port
		}
	}
	return ServerConfig{
		ListenAddr:            addr,
		HostKeyPath:           os.Getenv("SSH_HOST_KEY_PATH"),
		KnownHosts:            os.Getenv("SSH_KNOWN_HOSTS"),
		ServerIdent:           getenvDefault("SSH_SERVER_IDENT", "SSH-2.0-ssh-reverseproxy"),
		DialTimeout:           15 * time.Second,
		AcceptUnknownUpstream: getenvBool("SSH_ACCEPT_UNKNOWN_UPSTREAM", false),
		DBDSN:                 os.Getenv("SSH_DB_DSN"),
		DBTable:               getenvDefault("SSH_DB_TABLE", "proxy_mappings"),
		DBTimeout:             getenvDuration("SSH_DB_TIMEOUT", 10*time.Second),
		DBRefreshInterval:     getenvDuration("SSH_DB_REFRESH_INTERVAL", 30*time.Second),
	}
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getenvDuration(k string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
