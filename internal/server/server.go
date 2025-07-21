package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/handlers"
	"github.com/YuarenArt/marketgo/internal/logging"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"time"

	_ "github.com/YuarenArt/marketgo/docs"
)

// Server представляет HTTP-сервер с роутером Gin и логгированием
type Server struct {
	router    *gin.Engine
	logger    logging.Logger
	apiLogger logging.Logger
	config    *config.Config
	handler   *handlers.Handler
}

// NewServer создаёт новый экземпляр Server
func NewServer(cfg *config.Config, logger, apiLogger logging.Logger, handler *handlers.Handler) *Server {
	r := gin.New()
	s := &Server{
		router:    r,
		logger:    logger,
		apiLogger: apiLogger,
		config:    cfg,
		handler:   handler,
	}

	r.Use(s.loggingMiddleware, s.corsMiddleware(), gin.Recovery())
	s.setupRoutes()

	return s
}

// Start запускает HTTP-сервер и обрабатывает его завершение
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%s", s.config.Port)
	s.logger.Info("Starting server", "addr", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Server failed", "error", err)
		}
	}()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("Shutting down server...")

	if err := srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("Server forced to shutdown", "error", err)
		return err
	}

	s.logger.Info("Server stopped gracefully")
	return nil
}

func (s *Server) setupRoutes() {
	s.router.POST("/register", s.handler.Register)
	s.router.POST("/login", s.handler.Login)

	ads := s.router.Group("/ads", s.handler.AuthMiddleware())
	{
		ads.POST("", s.handler.CreateAd)
		ads.GET("", s.handler.Ads)
	}

	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// loggingMiddleware логирует каждый HTTP-запрос
func (s *Server) loggingMiddleware(c *gin.Context) {
	start := time.Now()
	method := c.Request.Method
	path := c.Request.URL.Path

	c.Next()

	latency := time.Since(start)
	status := c.Writer.Status()

	s.apiLogger.Info("HTTP request",
		"method", method,
		"path", path,
		"status", status,
		"duration", latency,
	)
}

// corsMiddleware добавляет заголовки для CORS
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
