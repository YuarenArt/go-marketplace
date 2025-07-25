package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/server/handlers"
	"github.com/YuarenArt/marketgo/pkg/logging"
	"github.com/YuarenArt/marketgo/pkg/metrics"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"net/http/pprof"
	"time"

	_ "github.com/YuarenArt/marketgo/docs"
)

var (
	excludedPaths = []string{"/metrics", "/debug/pprof/*"}
)

// Server представляет HTTP-сервер с роутером Gin и логгированием
type Server struct {
	router    *gin.Engine
	logger    logging.Logger
	apiLogger logging.Logger
	config    *config.Config
	handler   *handlers.Handler
	metrics   *metrics.Metrics
}

// NewServer создаёт новый экземпляр Server
func NewServer(cfg *config.Config, logger, apiLogger logging.Logger, handler *handlers.Handler, m *metrics.Metrics) *Server {
	r := gin.New()
	s := &Server{
		router:    r,
		logger:    logger,
		apiLogger: apiLogger,
		config:    cfg,
		handler:   handler,
		metrics:   m,
	}

	r.Use(
		s.loggingMiddleware,
		s.corsMiddleware(),
		gin.Recovery(),
		s.metrics.Middleware(),
		gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths(excludedPaths)),
	)
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

// setupRoutes настраивает маршруты HTTP-сервера.
// Регистрирует эндпоинты для:
// - Регистрации (/register)
// - Входа (/login)
// - Работы с объявлениями (/ads)
// - Swagger-документации (/swagger/*any)
// - Профилирования (/debug/pprof/*any, /debug/pprof/cmdline, /debug/pprof/profile, /debug/pprof/symbol, /debug/pprof/trace)
// - Метрик Prometheus (/metrics)
func (s *Server) setupRoutes() {

	s.router.POST("/register", s.handler.Register)
	s.router.POST("/login", s.handler.Login)

	ads := s.router.Group("/ads", s.handler.AuthMiddleware())
	{
		ads.POST("", s.handler.CreateAd)
		ads.GET("", s.handler.Ads)
	}

	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Маршруты для профилирования
	s.router.GET("/debug/pprof/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
	s.router.GET("/debug/pprof/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
	s.router.GET("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
	s.router.GET("/debug/pprof/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))

	s.setupMetrics()

}

// setupMetrics настраивает маршрут для метрик Prometheus
// @Summary Метрики Prometheus
// @Description Возвращает метрики приложения в формате Prometheus. Не использует Gzip-компрессию.
// @Tags metrics
// @Produce text/plain
// @Success 200 {string} string
// @Router /metrics [get]
func (s *Server) setupMetrics() {
	s.router.GET("/metrics", metrics.Handler())
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
