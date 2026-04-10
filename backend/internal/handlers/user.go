package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/middleware"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"github.com/thorved/ssh-reverseproxy/backend/internal/sshkeys"
	"gorm.io/gorm"
)

type UserHandler struct {
	cfg config.Config
	db  *gorm.DB
}

type listInstancesResponse struct {
	Instances []models.Instance `json:"instances"`
	SSHHost   string            `json:"ssh_host"`
	SSHPort   int               `json:"ssh_port"`
}

type sshKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
	IsActive  *bool  `json:"is_active"`
}

func NewUserHandler(cfg config.Config, db *gorm.DB) *UserHandler {
	return &UserHandler{cfg: cfg, db: db}
}

func (h *UserHandler) ListInstances(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)

	var instances []models.Instance
	if err := h.db.
		Where("assigned_user_id = ? AND enabled = ?", user.ID, true).
		Order("name asc").
		Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, listInstancesResponse{
		Instances: instances,
		SSHHost:   h.cfg.SSHPublicHost,
		SSHPort:   h.cfg.SSHPort(),
	})
}

func (h *UserHandler) ListSSHKeys(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)

	var keys []models.SSHKey
	if err := h.db.Where("user_id = ?", user.ID).Order("created_at desc").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (h *UserHandler) CreateSSHKey(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)

	var req sshKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := parsedSSHKey(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := models.SSHKey{
		UserID:      user.ID,
		Name:        strings.TrimSpace(req.Name),
		PublicKey:   key.PublicKey,
		Fingerprint: key.Fingerprint,
		Algorithm:   key.Algorithm,
		Comment:     key.Comment,
		IsActive:    req.IsActive == nil || *req.IsActive,
	}

	if err := h.db.Create(&record).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, record)
}

func (h *UserHandler) UpdateSSHKey(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)

	var key models.SSHKey
	if err := h.db.Where("user_id = ?", user.ID).First(&key, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ssh key not found"})
		return
	}

	var req sshKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parsed, err := parsedSSHKey(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key.Name = strings.TrimSpace(req.Name)
	key.PublicKey = parsed.PublicKey
	key.Fingerprint = parsed.Fingerprint
	key.Algorithm = parsed.Algorithm
	key.Comment = parsed.Comment
	if req.IsActive != nil {
		key.IsActive = *req.IsActive
	}

	if err := h.db.Save(&key).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, key)
}

func (h *UserHandler) DeleteSSHKey(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)

	result := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).Delete(&models.SSHKey{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "ssh key not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func parsedSSHKey(req sshKeyRequest) (*sshkeys.ParsedKey, error) {
	key, err := sshkeys.ParseAuthorizedKey(req.PublicKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name is required")
	}
	return key, nil
}
