package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/codersaadi/go-micro/db"
	"github.com/codersaadi/go-micro/internal/handler"
	repository "github.com/codersaadi/go-micro/internal/respository"
	"github.com/codersaadi/go-micro/internal/service"
	"github.com/codersaadi/go-micro/pkg/micro"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

func getConfig() (*micro.Config, error) {
	// Define default config with your specified values
	config := &micro.Config{
		AppName:        "user-service",
		Port:           8080,
		LogLevel:       "info",
		MetricsEnabled: true,
		RateLimiter: micro.RateLimiterConfig{
			Enabled:      true,
			RequestsPerS: 10,
			Burst:        20,
			TTL:          time.Hour,
			Strategy:     "ip",
		},
		CORS: micro.CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"https://yourdomain.com", "https://app.yourdomain.com"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
			ExposedHeaders:   []string{"X-Request-ID", "X-Rate-Limit-Remaining"},
			AllowCredentials: true,
			MaxAge:           600,
		},
	}

	// Override defaults with any environment variables that are set
	if err := envconfig.Process("", config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	return config, nil
}

func BootstrapServer() {
	// Configure the application with rate limiter settings
	cfg, err := getConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	// Create the micro app
	app, err := micro.NewApp(cfg)
	if err != nil {
		panic("Failed to create application: " + err.Error())
	}

	// Initialize database pool
	pool, err := db.NewPostgresPool(context.Background(), cfg.DBDSN)
	if err != nil {
		app.Logger.Error("Failed to create database pool", zap.Error(err))
		return
	}
	defer pool.Close()

	// Initialize application layers
	// Handler --> Service ---> Repository --> Database
	userRepo := repository.NewUserRepository(pool, app.Logger)
	userService := service.NewUserService(userRepo, app.Logger)
	userHandler := handler.NewUserHandler(app, userService)
	// Register routes (Example Routes)
	app.POST("/register", micro.Handler(userHandler.Register))
	app.POST("/login", micro.Handler(userHandler.Login))
	app.GET("/users/{id}", micro.Handler(userHandler.GetUser))
	app.PUT("/users/{id}", micro.Handler(userHandler.UpdateUser))
	app.DELETE("/users/{id}", micro.Handler(userHandler.DeleteUser))

	// Register a rate limit info endpoint (optional)
	app.GET("/rate-limit-info", micro.Handler(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		info := map[string]interface{}{
			"enabled":             app.Config.RateLimiter.Enabled,
			"requests_per_second": app.Config.RateLimiter.RequestsPerS,
			"burst":               app.Config.RateLimiter.Burst,
			"strategy":            app.Config.RateLimiter.Strategy,
		}
		return app.JSON(w, http.StatusOK, info)
	}))

	// Start server
	if err := app.Start(); err != nil && err != http.ErrServerClosed {
		app.Logger.Error("Server failed to start", zap.Error(err))
	}
}
