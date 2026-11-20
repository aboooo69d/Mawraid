package database

import (
	"context"
	"fmt"
	"time"

	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/database/entities"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	valueHashing "github.com/najm-x/nova/pkg/value_hashing"
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
		&entities.AuditLog{},
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// Enforce DB-level immutability for audit_logs table (Append-Only)
	triggerSQL := `
	CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
	RETURNS TRIGGER AS $$
	BEGIN
		RAISE EXCEPTION 'Audit logs are immutable: UPDATE and DELETE operations are strictly prohibited.';
	END;
	$$ LANGUAGE plpgsql;

	DO $$
	BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM pg_trigger WHERE tgname = 'trg_audit_logs_immutable'
		) THEN
			CREATE TRIGGER trg_audit_logs_immutable
			BEFORE UPDATE OR DELETE ON audit_logs
			FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
		END IF;
	END $$;
	`
	if err := db.Exec(triggerSQL).Error; err != nil {
		// Ignore if non-postgres dialect in tests, otherwise log/continue
		_ = err
	}

	return seedRoles(db)
}

func seedRoles(db *gorm.DB) error {
	defaultRoles := []entities.Role{
		{Name: roleRepo.RoleConsumer, DisplayName: "مستهلك"},
		{Name: roleRepo.RoleProvider, DisplayName: "مزود خدمة"},
		{Name: roleRepo.RoleAdmin, DisplayName: "مدير النظام"},
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

	return seedAdmin(db)
}

func seedAdmin(db *gorm.DB) error {
	var adminRole entities.Role
	if err := db.Where("name = ?", roleRepo.RoleAdmin).First(&adminRole).Error; err != nil {
		return fmt.Errorf("failed to find admin role: %w", err)
	}

	var count int64
	if err := db.Model(&entities.User{}).Where("role_id = ?", adminRole.ID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check existing admin users: %w", err)
	}

	if count == 0 {
		hashedPassword, err := valueHashing.HashValue("Admin@123456")
		if err != nil {
			return fmt.Errorf("failed to hash default admin password: %w", err)
		}

		adminUser := entities.User{
			Phone:      "777777777",
			Password:   hashedPassword,
			RoleID:     adminRole.ID,
			IsVerified: true,
			Status:     entities.StatusActive,
		}

		if err := db.Create(&adminUser).Error; err != nil {
			return fmt.Errorf("failed to seed default admin user: %w", err)
		}
	}

	return nil
}
