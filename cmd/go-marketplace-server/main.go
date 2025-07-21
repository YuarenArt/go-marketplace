package main

import (
	"context"
	"fmt"
	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/YuarenArt/marketgo/internal/handlers"
	"github.com/YuarenArt/marketgo/internal/logging"
	"github.com/YuarenArt/marketgo/internal/server"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading .env")
	}

	cfg := config.NewConfig()
	appLogger := logging.NewLogger(cfg)
	apiLogger := logging.NewLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DB.User, cfg.DB.Password,
		cfg.DB.Host, cfg.DB.Port,
		cfg.DB.DBName,
	)

	handler, err := handlers.NewHandler(
		handlers.WithConfig(ctx, dsn, cfg,
			db.WithMaxConns(20),
			db.WithMinConns(5),
			db.WithConnMaxLifetime(30*time.Minute),
		),
	)
	if err != nil {
		appLogger.Error("Failed to initialize handler", "error", err)
		log.Fatal()
	}

	srv := server.NewServer(cfg, appLogger, apiLogger, handler)
	go func() {
		if err := srv.Start(ctx); err != nil {
			appLogger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	appLogger.Info("Server stopped")
}
