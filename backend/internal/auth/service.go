package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type Service struct {
	cfg        config.Config
	db         *gorm.DB
	provider   *oidc.Provider
	verifier   *oidc.IDTokenVerifier
	oauth      *oauth2.Config
	adminSet   map[string]struct{}
	stateStore *stateStore
}

type oidcClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Nonce   string `json:"nonce"`
}

type loginState struct {
	RedirectURL string
	Nonce       string
	ExpiresAt   time.Time
}

type stateStore struct {
	mu    sync.Mutex
	items map[string]loginState
}

func newStateStore() *stateStore {
	return &stateStore{items: make(map[string]loginState)}
}

func (s *stateStore) put(key string, value loginState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
}

func (s *stateStore) pop(key string) (loginState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.items[key]
	if ok {
		delete(s.items, key)
	}
	return value, ok
}

func (s *stateStore) cleanup(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range s.items {
		if now.After(value.ExpiresAt) {
			delete(s.items, key)
		}
	}
}

func NewService(cfg config.Config, db *gorm.DB) (*Service, error) {
	if cfg.OIDCIssuerURL == "" || cfg.OIDCClientID == "" || cfg.OIDCClientSecret == "" {
		return nil, errors.New("OIDC_ISSUER_URL, OIDC_CLIENT_ID, and OIDC_CLIENT_SECRET are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	service := &Service{
		cfg:      cfg,
		db:       db,
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID}),
		oauth: &oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       cfg.OIDCScopes,
		},
		adminSet:   make(map[string]struct{}, len(cfg.AdminEmails)),
		stateStore: newStateStore(),
	}

	for _, email := range cfg.AdminEmails {
		service.adminSet[strings.ToLower(email)] = struct{}{}
	}

	go service.runStateCleanup()

	return service, nil
}

func (s *Service) oauthConfig(redirectURL string) *oauth2.Config {
	cfg := *s.oauth
	if strings.TrimSpace(redirectURL) != "" {
		cfg.RedirectURL = strings.TrimSpace(redirectURL)
	}
	return &cfg
}

func (s *Service) runStateCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.stateStore.cleanup(time.Now())
	}
}

func (s *Service) BeginLogin(redirectURL string) string {
	state := uuid.NewString()
	nonce := randomToken(24)
	s.stateStore.put(state, loginState{
		RedirectURL: redirectURL,
		Nonce:       nonce,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	})
	return s.oauthConfig(redirectURL).AuthCodeURL(state, oidc.Nonce(nonce))
}

func (s *Service) HandleCallback(ctx context.Context, code, state, redirectURL string) (*models.User, string, error) {
	savedState, ok := s.stateStore.pop(state)
	if !ok || time.Now().After(savedState.ExpiresAt) {
		return nil, "", errors.New("invalid or expired oidc state")
	}

	if strings.TrimSpace(savedState.RedirectURL) != "" {
		redirectURL = savedState.RedirectURL
	}

	token, err := s.oauthConfig(redirectURL).Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, "", errors.New("provider response missing id_token")
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", fmt.Errorf("verify id token: %w", err)
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, "", fmt.Errorf("decode oidc claims: %w", err)
	}

	if claims.Subject == "" || strings.TrimSpace(claims.Email) == "" {
		return nil, "", errors.New("oidc claims missing subject or email")
	}

	if claims.Nonce != "" && claims.Nonce != savedState.Nonce {
		return nil, "", errors.New("oidc nonce mismatch")
	}

	user, err := s.findOrCreateUser(ctx, claims)
	if err != nil {
		return nil, "", err
	}

	sessionToken, err := s.CreateSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionToken, nil
}

func (s *Service) findOrCreateUser(ctx context.Context, claims oidcClaims) (*models.User, error) {
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	name := strings.TrimSpace(claims.Name)

	var user models.User
	err := s.db.WithContext(ctx).
		Where("o_id_c_subject = ?", claims.Subject).
		Or("LOWER(email) = ?", email).
		First(&user).Error

	role := models.RoleUser
	if _, ok := s.adminSet[email]; ok {
		role = models.RoleAdmin
	}

	now := time.Now()

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		user = models.User{
			Email:       email,
			DisplayName: name,
			Role:        role,
			IsActive:    true,
			OIDCSubject: claims.Subject,
			LastLoginAt: &now,
		}
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("load user: %w", err)
	default:
		user.Email = email
		user.DisplayName = firstNonEmpty(name, user.DisplayName)
		user.OIDCSubject = claims.Subject
		user.LastLoginAt = &now
		if _, ok := s.adminSet[email]; ok {
			user.Role = models.RoleAdmin
		}
		if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	if !user.IsActive {
		return nil, errors.New("user is inactive")
	}

	return &user, nil
}

func (s *Service) CreateSession(ctx context.Context, userID uint) (string, error) {
	session := models.Session{
		UserID:    userID,
		Token:     randomToken(48),
		ExpiresAt: time.Now().Add(s.cfg.SessionTTL),
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return session.Token, nil
}

func (s *Service) GetUserBySession(ctx context.Context, token string) (*models.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var session models.Session
	err := s.db.WithContext(ctx).
		Preload("User").
		Where("token = ? AND expires_at > ?", token, time.Now()).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session.User, nil
}

func (s *Service) DeleteSession(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.db.WithContext(ctx).Where("token = ?", token).Delete(&models.Session{}).Error
}

func (s *Service) LogoutURL() string {
	u, err := url.Parse(s.cfg.FrontendBaseURL)
	if err != nil {
		return "/login"
	}
	return u.String() + "/login"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
