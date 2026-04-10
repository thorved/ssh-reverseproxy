package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db *gorm.DB
}

type createUserRequest struct {
	Email       string          `json:"email" binding:"required,email"`
	DisplayName string          `json:"display_name"`
	Role        models.UserRole `json:"role" binding:"required,oneof=admin user"`
	IsActive    *bool           `json:"is_active"`
}

type updateUserRequest struct {
	DisplayName *string          `json:"display_name"`
	Role        *models.UserRole `json:"role" binding:"omitempty,oneof=admin user"`
	IsActive    *bool            `json:"is_active"`
}

type instanceRequest struct {
	Name           string `json:"name" binding:"required"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	UpstreamHost   string `json:"upstream_host" binding:"required"`
	UpstreamPort   int    `json:"upstream_port"`
	UpstreamUser   string `json:"upstream_user" binding:"required"`
	AuthMethod     string `json:"auth_method" binding:"required,oneof=none password key"`
	AuthPassword   string `json:"auth_password"`
	AuthKeyInline  string `json:"auth_key_inline"`
	AuthPassphrase string `json:"auth_passphrase"`
	Enabled        *bool  `json:"enabled"`
	AssignedUserID *uint  `json:"assigned_user_id"`
}

type assignInstanceRequest struct {
	UserID *uint `json:"user_id"`
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Order("email asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]models.AuthUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, user.ToAuthResponse())
	}
	c.JSON(http.StatusOK, response)
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := models.User{
		Email:       strings.ToLower(strings.TrimSpace(req.Email)),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Role:        req.Role,
		IsActive:    true,
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user.ToAuthResponse())
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	var user models.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DisplayName != nil {
		user.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user.ToAuthResponse())
}

func (h *AdminHandler) ListInstances(c *gin.Context) {
	var instances []models.Instance
	if err := h.db.Preload("AssignedUser").Order("name asc").Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instances)
}

func (h *AdminHandler) CreateInstance(c *gin.Context) {
	instance, err := bindInstanceRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Create(instance).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Preload("AssignedUser").First(instance, instance.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, instance)
}

func (h *AdminHandler) UpdateInstance(c *gin.Context) {
	var instance models.Instance
	if err := h.db.First(&instance, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	applyInstanceRequest(&instance, req)
	if err := h.db.Save(&instance).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Preload("AssignedUser").First(&instance, instance.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instance)
}

func (h *AdminHandler) AssignInstance(c *gin.Context) {
	var instance models.Instance
	if err := h.db.First(&instance, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	var req assignInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID != nil {
		var user models.User
		if err := h.db.First(&user, *req.UserID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "assigned user not found"})
			return
		}
		instance.AssignedUserID = req.UserID
	} else {
		instance.AssignedUserID = nil
	}

	if err := h.db.Save(&instance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Preload("AssignedUser").First(&instance, instance.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instance)
}

func bindInstanceRequest(c *gin.Context) (*models.Instance, error) {
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	instance := &models.Instance{}
	applyInstanceRequest(instance, req)
	return instance, nil
}

func applyInstanceRequest(instance *models.Instance, req instanceRequest) {
	instance.Name = strings.TrimSpace(req.Name)
	instance.Slug = buildSlug(req.Slug, req.Name)
	instance.Description = strings.TrimSpace(req.Description)
	instance.UpstreamHost = strings.TrimSpace(req.UpstreamHost)
	if req.UpstreamPort > 0 {
		instance.UpstreamPort = req.UpstreamPort
	} else if instance.UpstreamPort == 0 {
		instance.UpstreamPort = 22
	}
	instance.UpstreamUser = strings.TrimSpace(req.UpstreamUser)
	instance.AuthMethod = strings.TrimSpace(req.AuthMethod)
	instance.AuthPassword = req.AuthPassword
	instance.AuthKeyInline = req.AuthKeyInline
	instance.AuthPassphrase = req.AuthPassphrase
	if req.Enabled != nil {
		instance.Enabled = *req.Enabled
	} else if instance.ID == 0 {
		instance.Enabled = true
	}
	instance.AssignedUserID = req.AssignedUserID
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func buildSlug(rawSlug, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(rawSlug))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(fallback))
	}
	value = slugRe.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "instance"
	}
	return value
}
