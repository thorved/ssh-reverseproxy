package models

import "time"

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

type User struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Email       string     `gorm:"uniqueIndex;not null" json:"email"`
	DisplayName string     `json:"display_name"`
	Role        UserRole   `gorm:"type:text;not null;default:user" json:"role"`
	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`
	OIDCSubject string     `gorm:"column:o_id_c_subject;uniqueIndex" json:"-"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	SSHKeys   []SSHKey   `json:"-"`
	Instances []Instance `gorm:"many2many:instance_assignments;" json:"-"`
}

type SSHKey struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Name        string    `gorm:"not null" json:"name"`
	PublicKey   string    `gorm:"not null" json:"public_key"`
	Fingerprint string    `gorm:"uniqueIndex;not null" json:"fingerprint"`
	Algorithm   string    `json:"algorithm"`
	Comment     string    `json:"comment"`
	IsActive    bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Instance struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"not null" json:"name"`
	Slug            string    `gorm:"uniqueIndex;not null" json:"slug"`
	Description     string    `json:"description"`
	AssignedUsers   []User    `gorm:"many2many:instance_assignments;" json:"assigned_users,omitempty"`
	AssignedUserIDs []uint    `gorm:"-" json:"assigned_user_ids"`
	UpstreamHost    string    `gorm:"not null" json:"upstream_host"`
	UpstreamPort    int       `gorm:"not null;default:22" json:"upstream_port"`
	UpstreamUser    string    `gorm:"not null" json:"upstream_user"`
	AuthMethod      string    `gorm:"not null;default:key" json:"auth_method"`
	AuthPassword    string    `json:"auth_password,omitempty"`
	AuthKeyInline   string    `json:"auth_key_inline,omitempty"`
	AuthPassphrase  string    `json:"auth_passphrase,omitempty"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type InstanceAssignment struct {
	InstanceID uint      `gorm:"primaryKey;autoIncrement:false" json:"instance_id"`
	UserID     uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Session struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `json:"-"`
}

type AuthUserResponse struct {
	ID          uint       `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Role        UserRole   `json:"role"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

func (u User) ToAuthResponse() AuthUserResponse {
	return AuthUserResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		IsActive:    u.IsActive,
		LastLoginAt: u.LastLoginAt,
	}
}

func (i *Instance) PopulateAssignedUserIDs() {
	i.AssignedUserIDs = make([]uint, 0, len(i.AssignedUsers))
	for _, user := range i.AssignedUsers {
		i.AssignedUserIDs = append(i.AssignedUserIDs, user.ID)
	}
}
