package main

import (
	"context"
	"fmt"
	"github.com/YuarenArt/marketgo/pkg/metrics"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/YuarenArt/marketgo/internal/server"
	"github.com/YuarenArt/marketgo/internal/server/handlers"
	"github.com/YuarenArt/marketgo/pkg/logging"
	"github.com/joho/godotenv"
)

// @title Marketplace API
// @version 1.0
// @description API для онлайн-маркетплейса с авторизацией и объявлениями
// @host localhost:8080
// @BasePath /
// @schemes http

// @securityDefinitions.apikey BearerAuth
// @in header
// @name X-Auth-Token

// @header Accept-Encoding {string} gzip "Указывает, что клиент поддерживает Gzip-компрессию"
// @header Content-Encoding {string} gzip "Указывает, что ответ сжат с использованием Gzip"

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env")
	}

	cfg := config.NewConfig()
	appLogger := logging.NewLogger(cfg)
	apiLogger := logging.NewFileLogger("logs/api.log")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DB.User, cfg.DB.Password,
		cfg.DB.Host, cfg.DB.Port,
		cfg.DB.DBName,
	)

	handler, err := handlers.NewHandler(
		handlers.WithLogger(appLogger),
		handlers.WithConfig(ctx, dsn, cfg,
			db.WithMaxConns(200),
			db.WithMinConns(20),
			db.WithConnMaxLifetime(30*time.Minute),
			db.WithConnIdleLifetime(5*time.Minute),
		),
	)
	if err != nil {
		appLogger.Error("Failed to initialize handler", "error", err)
		log.Fatal()
	}

	metrics := metrics.NewMetrics()
	srv := server.NewServer(cfg, appLogger, apiLogger, handler, metrics)
	go func() {
		if err := srv.Start(ctx); err != nil {
			appLogger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	appLogger.Info("Server stopped")
}
