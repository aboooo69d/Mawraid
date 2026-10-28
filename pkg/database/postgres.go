package database

import (
	"context"
	"fmt"
	"time"

	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/database/entities"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgres creates and configures a PostgreSQL connection via GORM.
func NewPostgres(cfg config.DatabaseConfig) (*gorm.DB, error) {
	logLevel := logger.Warn
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Quick ping to verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return db, nil
}

// AutoMigrateAndSeed creates tables and ensures standard roles exist.
func AutoMigrateAndSeed(db *gorm.DB) error {
	if err := db.Exec("DO $$ BEGIN CREATE TYPE user_status AS ENUM ('pending','docs','active','rejected','suspended'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;").Error; err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}
	err := db.AutoMigrate(
		&entities.Role{},
		&entities.Permission{},
		&entities.User{},
		&entities.Provider{},
		&entities.Consumer{},
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	return seedRoles(db)
}

func seedRoles(db *gorm.DB) error {
	defaultRoles := []entities.Role{
		{Name: roleRepo.RoleConsumer, DisplayName: "مستهلك"},
		{Name: roleRepo.RoleProvider, DisplayName: "مزود خدمة"},
	}

	for _, role := range defaultRoles {
		var count int64
		if err := db.Model(&entities.Role{}).Where("name = ?", role.Name).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check role %s: %w", role.Name, err)
		}
		if count == 0 {
			if err := db.Create(&role).Error; err != nil {
				return fmt.Errorf("failed to seed role %s: %w", role.Name, err)
			}
		}
	}

	return nil
}
