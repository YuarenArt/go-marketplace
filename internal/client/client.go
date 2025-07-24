package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/YuarenArt/marketgo/internal/db"
	"github.com/YuarenArt/marketgo/internal/server/services"
	"github.com/YuarenArt/marketgo/pkg/logging"
)

// Константы для заголовков, путей API и сообщений об ошибках
const (
	contentType         = "Content-Type"
	acceptEncoding      = "Accept-Encoding"
	authHeader          = "X-Auth-Token"
	pathRegister        = "/register"
	pathLogin           = "/login"
	pathAds             = "/ads"
	jsonContentType     = "application/json"
	gzipEncoding        = "gzip"
	errMsgMarshalFailed = "Не удалось сериализовать данные"
	errMsgRequestFailed = "Не удалось выполнить запрос"
	errMsgGzipFailed    = "Не удалось обработать Gzip"
	errMsgDecodeFailed  = "Не удалось декодировать ответ"
)

// APIError представляет ошибку API с кодом статуса и сообщением
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("запрос не выполнен: %s (статус: %d)", e.Message, e.StatusCode)
}

// Client представляет HTTP-клиент для выполнения API-запросов
type Client struct {
	client  *http.Client
	logger  logging.Logger
	baseURL string
	token   string
}

// NewClient создает новый HTTP-клиент с заданной базовой URL и логгером
func NewClient(baseURL string, logger logging.Logger) *Client {
	return &Client{
		client: &http.Client{
			Timeout: 10 * time.Second, // Таймаут 10 секунд
		},
		logger:  logger,
		baseURL: baseURL,
	}
}

// SetToken обновляет токен авторизации клиента
func (c *Client) SetToken(token string) {
	c.token = token
}

// marshalBody сериализует данные в JSON
func marshalBody(data interface{}) ([]byte, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("сериализация: %w", err)
	}
	return body, nil
}

// doRequest выполняет HTTP-запрос и декодирует ответ
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, useAuth bool, result interface{}, logContext ...interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		c.logger.Error(errMsgRequestFailed, append(logContext, "error", err)...)
		return fmt.Errorf("создание запроса: %w", err)
	}

	req.Header.Set(contentType, jsonContentType)
	req.Header.Set(acceptEncoding, gzipEncoding)
	if useAuth {
		req.Header.Set(authHeader, c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error(errMsgRequestFailed, append(logContext, "error", err)...)
		return fmt.Errorf("отправка запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp["error"]
		if msg == "" {
			msg = fmt.Sprintf("код статуса %d", resp.StatusCode)
		}
		c.logger.Error(errMsgRequestFailed, append(logContext, "status", resp.StatusCode, "error", msg)...)
		return &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	reader := resp.Body
	if resp.Header.Get("Content-Encoding") == gzipEncoding {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.logger.Error(errMsgGzipFailed, append(logContext, "error", err)...)
			return fmt.Errorf("Gzip: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	if err := json.NewDecoder(reader).Decode(result); err != nil {
		c.logger.Error(errMsgDecodeFailed, append(logContext, "error", err)...)
		return fmt.Errorf("декодирование: %w", err)
	}

	return nil
}

// Register регистрирует нового пользователя
func (c *Client) Register(ctx context.Context, input *services.InputUserInfo) (db.User, error) {
	if input == nil || input.Login == "" {
		c.logger.Error("Некорректный логин", "login", input.Login)
		return db.User{}, errors.New("логин не указан")
	}

	body, err := marshalBody(input)
	if err != nil {
		c.logger.Error(errMsgMarshalFailed, "login", input.Login, "error", err)
		return db.User{}, err
	}

	var user db.User
	if err := c.doRequest(ctx, http.MethodPost, pathRegister, bytes.NewBuffer(body), false, &user, "login", input.Login); err != nil {
		return db.User{}, err
	}

	c.logger.Info("Регистрация успешна", "login", input.Login, "user_id", user.ID)
	return user, nil
}

// Login аутентифицирует пользователя и сохраняет токен
func (c *Client) Login(ctx context.Context, input *services.InputUserInfo) error {
	if input == nil || input.Login == "" {
		c.logger.Error("Некорректный логин", "login", input.Login)
		return errors.New("логин не указан")
	}

	body, err := marshalBody(input)
	if err != nil {
		c.logger.Error(errMsgMarshalFailed, "login", input.Login, "error", err)
		return err
	}

	var result struct {
		Token string `json:"token"`
	}
	err = c.doRequest(ctx, http.MethodPost, pathLogin, bytes.NewBuffer(body), false, &result, "login", input.Login)
	if err != nil {
		return err
	}

	c.SetToken(result.Token)
	c.logger.Info("Вход успешен", "login", input.Login)
	return nil
}

// PostAdd создает новое объявление
func (c *Client) PostAdd(ctx context.Context, adReq *services.CreateAdRequest) (db.Ad, error) {
	if adReq == nil || adReq.Title == "" {
		c.logger.Error("Некорректный заголовок объявления", "title", adReq.Title)
		return db.Ad{}, errors.New("заголовок не указан")
	}

	body, err := marshalBody(adReq)
	if err != nil {
		c.logger.Error(errMsgMarshalFailed, "title", adReq.Title, "error", err)
		return db.Ad{}, err
	}

	var ad db.Ad
	if err := c.doRequest(ctx, http.MethodPost, pathAds, bytes.NewBuffer(body), true, &ad, "title", adReq.Title); err != nil {
		return db.Ad{}, err
	}

	c.logger.Info("Объявление создано", "ad_id", ad.ID, "title", ad.Title)
	return ad, nil
}

// GetAds получает список объявлений с фильтрацией и сортировкой
func (c *Client) GetAds(ctx context.Context, req services.GetAdsRequest) ([]db.Ad, error) {
	if req.Page < 1 || req.PageSize < 1 || req.PageSize > 100 {
		c.logger.Error("Некорректные параметры", "page", req.Page, "page_size", req.PageSize)
		return nil, fmt.Errorf("некорректные параметры: page=%d, page_size=%d", req.Page, req.PageSize)
	}

	query := url.Values{
		"page":      []string{strconv.Itoa(req.Page)},
		"page_size": []string{strconv.Itoa(req.PageSize)},
	}
	if req.SortBy != "" {
		query.Set("sort_by", req.SortBy)
	}
	if req.SortOrder != "" {
		query.Set("sort_order", req.SortOrder)
	}
	if req.MinPrice > 0 {
		query.Set("min_price", strconv.FormatInt(req.MinPrice, 10))
	}
	if req.MaxPrice > 0 {
		query.Set("max_price", strconv.FormatInt(req.MaxPrice, 10))
	}

	var ads []db.Ad
	if err := c.doRequest(ctx, http.MethodGet, pathAds+"?"+query.Encode(), nil, true, &ads, "page", req.Page); err != nil {
		return nil, err
	}

	c.logger.Info("Объявления получены", "page", req.Page, "count", len(ads))
	return ads, nil
}
