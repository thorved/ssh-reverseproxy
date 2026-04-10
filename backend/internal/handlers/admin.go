package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/middleware"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"github.com/thorved/ssh-reverseproxy/backend/internal/sshkeys"
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
	Name              string `json:"name" binding:"required"`
	Slug              string `json:"slug"`
	Description       string `json:"description"`
	UpstreamHost      string `json:"upstream_host" binding:"required"`
	UpstreamPort      int    `json:"upstream_port"`
	UpstreamUser      string `json:"upstream_user" binding:"required"`
	AuthMethod        string `json:"auth_method" binding:"required,oneof=none password key"`
	AuthPassword      string `json:"auth_password"`
	AuthKeyInline     string `json:"auth_key_inline"`
	AuthPassphrase    string `json:"auth_passphrase"`
	RegenerateAuthKey bool   `json:"regenerate_auth_key"`
	Enabled           *bool  `json:"enabled"`
	AssignedUserIDs   []uint `json:"assigned_user_ids"`
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

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	currentUser, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var user models.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.ID == currentUser.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admins cannot delete their own account"})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.InstanceAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.SSHKey{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) ListUserSSHKeys(c *gin.Context) {
	user, err := h.findUser(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var keys []models.SSHKey
	if err := h.db.Where("user_id = ?", user.ID).Order("created_at desc").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (h *AdminHandler) CreateUserSSHKey(c *gin.Context) {
	user, err := h.findUser(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

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

func (h *AdminHandler) DeleteUserSSHKey(c *gin.Context) {
	user, err := h.findUser(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	result := h.db.Where("id = ? AND user_id = ?", c.Param("keyId"), user.ID).Delete(&models.SSHKey{})
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

func (h *AdminHandler) ListInstances(c *gin.Context) {
	var instances []models.Instance
	if err := h.preloadedInstancesQuery().
		Order("name asc").
		Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for index := range instances {
		h.populateInstanceResponse(&instances[index])
	}
	c.JSON(http.StatusOK, instances)
}

func (h *AdminHandler) CreateInstance(c *gin.Context) {
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instance := &models.Instance{}
	if err := applyInstanceRequest(instance, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(instance).Error; err != nil {
			return err
		}
		return h.replaceAssignedUsers(tx, instance, req.AssignedUserIDs)
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.loadInstance(instance, instance.ID); err != nil {
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

	if err := applyInstanceRequest(&instance, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&instance).Error; err != nil {
			return err
		}
		return h.replaceAssignedUsers(tx, &instance, req.AssignedUserIDs)
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.loadInstance(&instance, instance.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instance)
}

func (h *AdminHandler) DeleteInstance(c *gin.Context) {
	var instance models.Instance
	if err := h.db.First(&instance, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("instance_id = ?", instance.ID).Delete(&models.InstanceAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&instance).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) preloadedInstancesQuery() *gorm.DB {
	return h.db.Preload("AssignedUsers", func(db *gorm.DB) *gorm.DB {
		return db.Order("email asc")
	})
}

func (h *AdminHandler) loadInstance(instance *models.Instance, id uint) error {
	if err := h.preloadedInstancesQuery().First(instance, id).Error; err != nil {
		return err
	}
	h.populateInstanceResponse(instance)
	return nil
}

func (h *AdminHandler) populateInstanceResponse(instance *models.Instance) {
	instance.PopulateAssignedUserIDs()
	instance.AuthPublicKey = derivedInstancePublicKey(instance)
}

func (h *AdminHandler) replaceAssignedUsers(tx *gorm.DB, instance *models.Instance, rawIDs []uint) error {
	assignedUsers, err := h.usersByIDs(tx, rawIDs)
	if err != nil {
		return err
	}
	if err := tx.Model(instance).Association("AssignedUsers").Replace(assignedUsers); err != nil {
		return err
	}
	instance.AssignedUsers = assignedUsers
	instance.PopulateAssignedUserIDs()
	return nil
}

func (h *AdminHandler) usersByIDs(tx *gorm.DB, rawIDs []uint) ([]models.User, error) {
	ids := uniqueUintIDs(rawIDs)
	if len(ids) == 0 {
		return []models.User{}, nil
	}

	var users []models.User
	if err := tx.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) != len(ids) {
		return nil, errors.New("one or more assigned users were not found")
	}

	usersByID := make(map[uint]models.User, len(users))
	for _, user := range users {
		usersByID[user.ID] = user
	}

	orderedUsers := make([]models.User, 0, len(ids))
	for _, id := range ids {
		user, ok := usersByID[id]
		if !ok {
			return nil, errors.New("one or more assigned users were not found")
		}
		orderedUsers = append(orderedUsers, user)
	}
	return orderedUsers, nil
}

func (h *AdminHandler) findUser(rawID string) (*models.User, error) {
	var user models.User
	if err := h.db.First(&user, rawID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func uniqueUintIDs(rawIDs []uint) []uint {
	if len(rawIDs) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(rawIDs))
	ids := make([]uint, 0, len(rawIDs))
	for _, id := range rawIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func applyInstanceRequest(instance *models.Instance, req instanceRequest) error {
	previousAuthMethod := strings.TrimSpace(instance.AuthMethod)
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
	switch instance.AuthMethod {
	case "key":
		trimmedKey := strings.TrimSpace(req.AuthKeyInline)
		if req.RegenerateAuthKey || trimmedKey == "" {
			if req.RegenerateAuthKey || strings.TrimSpace(instance.AuthKeyInline) == "" {
				privateKey, _, passphrase, err := sshkeys.GeneratePrivateKeyPEM()
				if err != nil {
					return err
				}
				instance.AuthKeyInline = privateKey
				instance.AuthPassphrase = passphrase
			}
		} else {
			instance.AuthKeyInline = trimmedKey
			instance.AuthPassphrase = req.AuthPassphrase
		}
		instance.AuthPassword = ""
	case "password":
		if strings.TrimSpace(req.AuthPassword) != "" {
			instance.AuthPassword = req.AuthPassword
		} else if instance.ID == 0 || previousAuthMethod != "password" {
			return errors.New("upstream password is required")
		}
		instance.AuthKeyInline = ""
		instance.AuthPassphrase = ""
	case "none":
		instance.AuthPassword = ""
		instance.AuthKeyInline = ""
		instance.AuthPassphrase = ""
	}
	if req.Enabled != nil {
		instance.Enabled = *req.Enabled
	} else if instance.ID == 0 {
		instance.Enabled = true
	}
	return nil
}

func derivedInstancePublicKey(instance *models.Instance) string {
	if strings.TrimSpace(instance.AuthMethod) != "key" || strings.TrimSpace(instance.AuthKeyInline) == "" {
		return ""
	}

	publicKey, err := sshkeys.PublicKeyFromPrivateKey(instance.AuthKeyInline, instance.AuthPassphrase)
	if err != nil {
		return ""
	}
	return publicKey
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
