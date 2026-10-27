package appcontainer

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	authHandler "github.com/najm-x/nova/modules/auth/handler"
	authRepo "github.com/najm-x/nova/modules/auth/repo"
	authRouter "github.com/najm-x/nova/modules/auth/router"
	authService "github.com/najm-x/nova/modules/auth/service"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	userService "github.com/najm-x/nova/modules/user/service"
	"github.com/najm-x/nova/pkg/database"
	smsSender "github.com/najm-x/nova/pkg/sms_sender"
	"github.com/najm-x/nova/pkg/storage"
	"github.com/najm-x/nova/pkg/validator"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	_ "github.com/najm-x/nova/docs"
)

// AppContainer holds all application dependencies and handles dependency injection.
type AppContainer struct {
	Config      *config.Config
	DB          *gorm.DB
	Redis       *redis.Client
	Storage     storage.Storage
	Uploader    *storage.SecureUploader
	SmsProvider smsSender.Provider

	// Repositories
	UserRepo userRepo.UserRepo
	RoleRepo roleRepo.RoleRepo
	OtpRepo  authRepo.OtpRepo

	// Services
	OtpService  authService.OtpService
	AuthService authService.AuthService
	UserService userService.UserService

	// Handlers
	AuthHandler *authHandler.AuthHandler
}

// NewAppContainer wires together database, redis, storage, repositories, services, and handlers.
func NewAppContainer(ctx context.Context, cfg *config.Config) (*AppContainer, error) {
	// init validators
	validator.Init()

	// 1. Connect to PostgreSQL
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("appcontainer: postgres error: %w", err)
	}

	// 2. Auto-migrate schema and seed required roles
	if err := database.AutoMigrateAndSeed(db); err != nil {
		return nil, fmt.Errorf("appcontainer: migration error: %w", err)
	}

	// 3. Connect to Redis
	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("appcontainer: redis error: %w", err)
	}

	// 4. Validate Encryption Key for uploads
	encKey, err := cfg.EncryptionKeyBytes()
	if err != nil {
		return nil, fmt.Errorf("appcontainer: invalid encryption key: %w", err)
	}

	// 5. Initialize Storage & Secure Uploader
	r2Storage, err := storage.NewR2Storage(ctx, cfg.Cloudflare)
	if err != nil {
		return nil, fmt.Errorf("appcontainer: storage init error: %w", err)
	}
	uploader := storage.NewSecureUploader(r2Storage, encKey)

	// 6. External Providers
	smsClient := smsSender.NewSmsService()

	// 7. Repositories
	uRepo := userRepo.NewUserRepo(db)
	rRepo := roleRepo.NewRoleRepo(db)
	oRepo := authRepo.NewOtpRepo(rdb)

	// 8. Services
	oService := authService.NewOtpService(oRepo)
	aService := authService.NewAuthService(uRepo, rRepo, oService)
	uService := userService.NewUserService(uRepo)

	// 9. Handlers
	aHandler := authHandler.NewAuthHandler(aService, uploader)

	return &AppContainer{
		Config:      cfg,
		DB:          db,
		Redis:       rdb,
		Storage:     r2Storage,
		Uploader:    uploader,
		SmsProvider: smsClient,

		UserRepo: uRepo,
		RoleRepo: rRepo,
		OtpRepo:  oRepo,

		OtpService:  oService,
		AuthService: aService,
		UserService: uService,

		AuthHandler: aHandler,
	}, nil
}

// RegisterRoutes registers all modules, API groups and health checks on the Gin Engine.
func (c *AppContainer) RegisterRoutes(r *gin.Engine) {
	// Swagger UI documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
		})
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		authRouter.RegisterAuthRoutes(v1, c.AuthHandler)
	}
}

// Close gracefully terminates database and redis connections.
func (c *AppContainer) Close() {
	if c.DB != nil {
		if sqlDB, err := c.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if c.Redis != nil {
		_ = c.Redis.Close()
	}
}
