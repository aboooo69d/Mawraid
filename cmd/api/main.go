package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/najm-x/nova/config"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/pkg/appcontainer"
)

// @title           Nova API
// @version         1.0
// @description     Backend REST API for Nova platform with Go, Gin, PostgreSQL, and Redis.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Nova API Support
// @contact.email  support@nova.local

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1
func main() {
	// 1. Initialize configuration from environment / .env
	cfg := config.NewConfig()

	// 2. Setup context listening for OS interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Initialize DI Container (DB, Redis, S3, Repos, Services, Handlers)
	log.Println("[INFO] Initializing application dependencies...")
	container, err := appcontainer.NewAppContainer(ctx, cfg)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize AppContainer: %v", err)
	}
	defer container.Close()
	log.Println("[INFO] Connected to PostgreSQL, schema migrated & roles seeded successfully.")
	log.Println("[INFO] Connected to Redis successfully.")

	// 4. Setup Gin engine
	if cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(middleware.ErrorHandler())
	// 5. Register application routes
	container.RegisterRoutes(r)

	// 6. Configure HTTP Server
	serverAddr := ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 7. Start HTTP Server in background
	go func() {
		log.Printf("[INFO] Server is running on http://localhost%s\n", serverAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	}()

	// 8. Wait for termination signal
	<-ctx.Done()
	log.Println("[INFO] Shutting down server gracefully...")

	// 9. Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server exited successfully.")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
