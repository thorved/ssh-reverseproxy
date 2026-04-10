package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(cfg config.Config) (*gorm.DB, error) {
	dir := filepath.Dir(cfg.DatabasePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	logLevel := logger.Error
	if cfg.Env == "development" {
		logLevel = logger.Info
	}

	return gorm.Open(sqlite.Open(cfg.DatabasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.SSHKey{},
		&models.Instance{},
		&models.Session{},
	); err != nil {
		return err
	}

	for _, migration := range []struct {
		model  any
		fields []string
	}{
		{
			model: &models.User{},
			fields: []string{
				"DisplayName",
				"Role",
				"IsActive",
				"OIDCSubject",
				"LastLoginAt",
			},
		},
		{
			model: &models.SSHKey{},
			fields: []string{
				"Name",
				"PublicKey",
				"Fingerprint",
				"Algorithm",
				"Comment",
				"IsActive",
			},
		},
		{
			model: &models.Instance{},
			fields: []string{
				"Name",
				"Slug",
				"Description",
				"AssignedUserID",
				"UpstreamHost",
				"UpstreamPort",
				"UpstreamUser",
				"AuthMethod",
				"AuthPassword",
				"AuthKeyInline",
				"AuthPassphrase",
				"Enabled",
			},
		},
		{
			model: &models.Session{},
			fields: []string{
				"UserID",
				"Token",
				"ExpiresAt",
			},
		},
	} {
		for _, field := range migration.fields {
			if db.Migrator().HasColumn(migration.model, field) {
				continue
			}
			if err := db.Migrator().AddColumn(migration.model, field); err != nil {
				return fmt.Errorf("add missing column %s: %w", field, err)
			}
		}
	}

	return nil
}
