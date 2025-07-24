package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/YuarenArt/marketgo/internal/config"
	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/YuarenArt/marketgo/internal/server/services"
	"github.com/YuarenArt/marketgo/pkg/logging"
	"github.com/gin-gonic/gin"
)

const (
	AuthHeader       = "X-Auth-Token"
	ErrTokenRequired = "token required"
	ErrInvalidToken  = "invalid token"
	ErrUnauthorized  = "unauthorized"
	ErrInvalidCreds  = "invalid credentials"
)

// HandlerOption описывает функцию настройки Handler
type HandlerOption func(h *Handler) error

// Handler содержит бизнес-логику и доступ к сервисам
type Handler struct {
	authService *services.AuthService
	adService   *services.AdService
	logger      logging.Logger
}

// NewHandler создаёт Handler, применяя набор опций.
// Каждая опция конфигурирует или инициализирует часть Handler.
func NewHandler(opts ...HandlerOption) (*Handler, error) {
	h := &Handler{}
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return nil, err
		}
	}
	if h.logger == nil {
		h.logger = logging.NewLogger(nil)
	}
	return h, nil
}

// WithConfig инициализирует внутренние сервисы по конфигу
func WithConfig(ctx context.Context, dsn string, cfg *config.Config, dbOptions ...db.DBOption) HandlerOption {
	return func(h *Handler) error {

		logger := logging.NewLogger(cfg)
		dbSvc, err := db.NewDBService(ctx, dsn, dbOptions...)
		if err != nil {
			logger.Error("Failed to init DBService", "error", err)
			return err
		}

		h.authService = services.NewAuthService(dbSvc, cfg.JWTSecret)
		h.adService = services.NewAdService(dbSvc)
		h.logger = logger
		return nil
	}
}

// WithCustomDB позволяет передать готовый DBService вручную (без коннекта по DSN)
func WithCustomDB(dbSvc *db.DBService) HandlerOption {
	return func(h *Handler) error {
		h.authService = services.NewAuthService(dbSvc, "") // можно позже перезадать secret
		h.adService = services.NewAdService(dbSvc)
		return nil
	}
}

func WithLogger(l logging.Logger) HandlerOption {
	return func(h *Handler) error {
		h.logger = l
		return nil
	}
}

// AuthMiddleware проверяет JWT и устанавливает userID в контекст запроса
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader(AuthHeader)
		if header == "" {
			abortWithError(c, http.StatusUnauthorized, ErrTokenRequired)
			return
		}

		token := strings.TrimSpace(header)
		userID, err := h.authService.ValidateToken(token)
		if err != nil {
			abortWithError(c, http.StatusUnauthorized, ErrInvalidToken)
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// abortWithError - универсальная функция для возврата ошибки в JSON
func abortWithError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// Register регистрирует нового пользователя
// @Summary Регистрация пользователя
// @Description Регистрирует нового пользователя с указанным логином и паролем
// @Tags auth
// @Accept json
// @Produce json
// @Param input body services.InputUserInfo true "Данные пользователя"
// @Success 200 {object} db.User
// @Header 200 {string} Content-Encoding "gzip"
// @Failure 400 {object} map[string]string
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	h.logger.Debug("Register endpoint called")
	var input services.InputUserInfo
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("Register: invalid input", "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Debug("Register: input parsed", "login", input.Login)
	user, err := h.authService.Register(c, input)
	if err != nil {
		h.logger.Warn("Register: failed to register", "login", input.Login, "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Info("Register: user registered", "user_id", user.ID, "login", user.Login)
	c.JSON(http.StatusOK, user)
}

// Login аутентифицирует пользователя и возвращает токен
// @Summary Аутентификация пользователя
// @Description Аутентификация пользователя и возврат JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param input body services.InputUserInfo true "Данные пользователя"
// @Success 200 {object} map[string]string
// @Header 200 {string} Content-Encoding "gzip"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	h.logger.Debug("Login endpoint called")
	var input services.InputUserInfo
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("Login: invalid input", "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Debug("Login: input parsed", "login", input.Login)
	token, err := h.authService.Authenticate(c, input)
	if err != nil {
		h.logger.Warn("Login: authentication failed", "login", input.Login, "error", err)
		abortWithError(c, http.StatusUnauthorized, ErrInvalidCreds)
		return
	}

	h.logger.Info("Login: user authenticated", "login", input.Login)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// CreateAd создаёт объявление от авторизованного пользователя
// @Summary Создание объявления
// @Description Создаёт объявление от имени авторизованного пользователя
// @Tags ads
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body services.CreateAdRequest true "Данные объявления"
// @Success 200 {object} db.Ad
// @Header 200 {string} Content-Encoding "gzip"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /ads [post]
// @Security BearerAuth
func (h *Handler) CreateAd(c *gin.Context) {
	h.logger.Debug("CreateAd endpoint called")
	userID, ok := c.Get("userID")
	if !ok {
		h.logger.Warn("CreateAd: unauthorized access")
		abortWithError(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	var req services.CreateAdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("CreateAd: invalid input", "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Debug("CreateAd: input parsed", "user_id", userID, "title", req.Title)
	ad, err := h.adService.CreateAd(c, req, userID.(int))
	if err != nil {
		h.logger.Warn("CreateAd: failed to create ad", "user_id", userID, "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Info("CreateAd: ad created", "ad_id", ad.ID, "user_id", ad.UserID, "title", ad.Title)
	c.JSON(http.StatusOK, ad)
}

// Ads возвращает список объявлений с фильтрацией
// @Summary Получение списка объявлений
// @Description Возвращает список объявлений с фильтрами и сортировкой
// @Tags ads
// @Produce json
// @Security BearerAuth
// @Param page query int false "Номер страницы" default(1)
// @Param page_size query int false "Размер страницы" default(10)
// @Param sort_by query string false "Поле сортировки" default(created_at)
// @Param sort_order query string false "Порядок сортировки" default(DESC)
// @Param min_price query number false "Минимальная цена"
// @Param max_price query number false "Максимальная цена"
// @Success 200 {array} db.Ad
// @Header 200 {string} Content-Encoding "gzip"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /ads [get]
// @Security BearerAuth
func (h *Handler) Ads(c *gin.Context) {
	h.logger.Debug("Ads endpoint called")
	userID, ok := c.Get("userID")
	if !ok {
		h.logger.Warn("Ads: unauthorized access")
		abortWithError(c, http.StatusUnauthorized, ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "DESC")

	var minPrice, maxPrice int64
	if minStr := c.Query("min_price"); minStr != "" {
		minPrice, _ = strconv.ParseInt(minStr, 10, 64)
	}
	if maxStr := c.Query("max_price"); maxStr != "" {
		maxPrice, _ = strconv.ParseInt(maxStr, 10, 64)
	}

	h.logger.Debug("Ads: params", "user_id", userID, "page", page, "page_size", pageSize, "sort_by", sortBy, "sort_order", sortOrder, "min_price", minPrice, "max_price", maxPrice)

	req := services.GetAdsRequest{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		MinPrice:  minPrice,
		MaxPrice:  maxPrice,
	}

	ads, err := h.adService.GetAds(c, req, userID.(int))
	if err != nil {
		h.logger.Warn("Ads: failed to fetch ads", "user_id", userID, "error", err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Info("Ads: ads fetched", "count", len(ads), "user_id", userID)
	c.JSON(http.StatusOK, ads)
}

func (h *Handler) Log(level slog.Level, msg string, args ...interface{}) {
	if h.logger != nil {
		h.logger.Log(level, msg, args...)
	}
}
